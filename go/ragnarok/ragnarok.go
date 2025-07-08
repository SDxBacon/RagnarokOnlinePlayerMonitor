package ragnarok

import (
	"bytes"
	"fmt"
)

// +-------------------------------------------------------------------+
// | Offset | Field                    |   Length  | Value             |
// +-------------------------------------------------------------------+
// | 0x00   | Server Info Header       |   2 bytes | 0xc0a8            |
// | 0x02   | Don't Care (1)           |   2 bytes | (Arbitrary)       |
// | 0x04   | Server Name Indicator    |   2 bytes | 0x2d27 (variable) |
// | 0x06   | Server Name              |  20 bytes | (Server Name)     |
// | 0x1A   | Server Population        |   4 bytes | (Population)      |
// | 0x1E   | Server URL Indicator     |   2 bytes | 0x2712 (variable) |
// | 0x20   | Server URL               | 128 bytes | (Server URL)      |
// | 0xA0   | Don't Care (2)           |   4 bytes | (Arbitrary)       |
// +-------------------------------------------------------------------+
// Total length: 0xA4 (164 bytes)
//
//  NOTE: The server is running on a little endian architecture and 32-bit OS.

var charServerNameOffset int = 0x06
var charServerNameLength int = 20
var charServerPlayersOffset int = 0x1A
var charServerPlayersLength int = 4
var charServerUrlOffset int = 0x20
var charServerUrlLength int = 0x80 // 128 bytes

type CharacterServerInfo struct {
	Name    string
	Url     string
	Players int // TODO: use int32
}

func decodeServerData(data []byte) *CharacterServerInfo {
	// Extract the server name
	// - offset = 0x06
	// - length = 20
	start := charServerNameOffset
	end := start + charServerNameLength
	if end > len(data) {
		end = len(data)
	}
	serverNameBytes := data[start:end]
	serverNameBytes = bytes.Trim(serverNameBytes, "\x00") // trim the server name bytes
	serverName, err := DecodeBig5ToUTF8(serverNameBytes)  // convert the server name bytes from Big5 to UTF-8

	if err != nil {
		fmt.Println("轉換錯誤:", err)
		return nil
	}

	// Extract the server players
	// - offset = 0x1A
	// - length = 4
	//
	// NOTE: the bytes are in little endian format
	start = charServerPlayersOffset
	end = start + charServerPlayersLength
	if end > len(data) {
		end = len(data)
	}

	serverPlayersBytes := data[start:end]
	// convert the bytes to int in little endian format
	serverPlayers := int(BytesToInt32(serverPlayersBytes))

	// Extract the server URL
	// - offset = 0x20
	// - length = 127
	start = charServerUrlOffset
	end = start + charServerUrlLength
	if end > len(data) {
		end = len(data)
	}
	serverUrlBytes := data[start:end]
	serverUrlBytes = bytes.Trim(serverUrlBytes, "\x00") // trim the server URL bytes
	serverUrl := string(serverUrlBytes)

	return &CharacterServerInfo{
		Name:    serverName,
		Url:     serverUrl,
		Players: serverPlayers,
	}
}

// ParsePayloadToCharacterServerInfo processes a byte slice containing network packet data by searching for specific patterns.
// It takes two parameters:
//   - data: The raw byte slice containing the packet data to be parsed
//   - pattern: A byte slice containing the pattern to search for within the data
//
// The function performs the following operations:
// 1. Finds all occurrences of the given pattern in the data
// 2. Checks if the data length is evenly divisible by the number of pattern occurrences
// 3. If divisible, splits the data into equal segments starting at each pattern occurrence
//
// Returns:
//   - []byte: Combined segments if the data could be evenly split based on pattern occurrences
//   - nil: If the data length is not evenly divisible by the number of pattern occurrences
func ParsePayloadToCharacterServerInfo(data []byte, pattern []byte) []CharacterServerInfo {

	// find all occurrences of the pattern in the data
	occurrences := findAllPatterns(data, pattern)

	// debug information
	fmt.Printf("find %d occurrences of the pattern\n", len(occurrences))
	fmt.Println("occurrences:", occurrences)

	// if no occurrences found
	if len(occurrences) == 0 {
		return nil
	}

	// assuming the character server list is evenly distributed across the data, but may not start at the beginning.
	var dataCountFromOccurrence = len(data[occurrences[0]:]) // the number of bytes from the first occurrence to the end of the data
	if dataCountFromOccurrence <= 0 || len(data)%dataCountFromOccurrence != 0 {
		return nil
	}

	totalServers := len(occurrences)
	bytesPerServer := len(data[occurrences[0]:]) / totalServers

	serversBytes := make([]CharacterServerInfo, 0, totalServers)

	for i := 0; i < len(occurrences); i++ {
		start := occurrences[i]
		end := start + bytesPerServer
		// clamp end to data length
		if end > len(data) {
			end = len(data)
		}

		serverBytes := data[start:end]                  // slice the data from start to end
		charServerInfo := decodeServerData(serverBytes) // decode the server data

		// fmt.Printf("部分 %d (從位置 %d 開始):\n", i+1, start)
		// fmt.Println(hex.Dump(serverBytes))

		if charServerInfo == nil {
			// TODO:
			continue
		}

		serversBytes = append(serversBytes, *charServerInfo)
	}

	return serversBytes
}

// findAllPatterns finds all occurrences of a pattern in the data
func findAllPatterns(data []byte, pattern []byte) []int {
	var positions []int

	for i := 0; i <= len(data)-len(pattern); i++ {
		if bytes.Equal(data[i:i+len(pattern)], pattern) {
			positions = append(positions, i)
		}
	}

	return positions
}
