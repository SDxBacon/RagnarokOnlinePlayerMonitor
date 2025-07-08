package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"ro-server-player-monitor/go/config"
	"ro-server-player-monitor/go/github"
	"ro-server-player-monitor/go/network"
	"ro-server-player-monitor/go/ragnarok"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx                  context.Context
	services             AppServices
	isCapturing          bool
	packetCaptureService *network.PacketCaptureService

	appVersion string
}

type AppServices struct {
	github *github.GitHubService
}

// expose the LoginServer type from config package to js level
type LoginServer = config.LoginServer

// expose the CharacterServerInfo type from ragnarok package to js level
type CharacterServerInfo = ragnarok.CharacterServerInfo

var loginServers []LoginServer = []LoginServer{
	{Name: "Taiwan", IP: "219.84.200.54", Port: 6900, Pattern: []byte{0xc0, 0xa8}, IsNumberResponse: true},
	{Name: "Taiwan - Zero", IP: "35.229.252.108", Port: 6900, Pattern: []byte{0xc0, 0xa8}, IsNumberResponse: false},
	// {Name: "Korea", IP: "112.175.128.137", Port: 6900, Pattern: []byte{0xc0, 0xa8}},
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize services
	a.services = AppServices{
		github: github.NewGitHubService(ctx),
	}
	// build the config path
	configPath := a.buildConfigPath()
	runtime.LogInfo(a.ctx, "[App.startup] Config path: "+configPath)

	// Load the config file
	customServers, err := config.LoadCustomServersFromXML("./config.xml")
	if err != nil {
		runtime.LogInfof(a.ctx, "[App.startup] Failed to load config: %v", err)
		return
	}

	if len(customServers) == 0 {
		runtime.LogInfo(a.ctx, "[App.startup] No custom servers found in config.")
		return
	}

	// Custom servers are found, merge them with default servers
	for _, customServer := range customServers {
		// Check if the server already exists in the default list
		var ptr *LoginServer

		for i, server := range loginServers {
			if customServer.Name == server.Name {
				ptr = &loginServers[i]
				break
			}
		}

		if ptr == nil {
			// If the server doesn't exist, append it to the list
			loginServers = append(loginServers, customServer)
		} else {
			// Otherwise, update the existing server with the custom server's details
			*ptr = customServer
		}
	}

	runtime.LogInfof(a.ctx, "[App.startup] customServers: %+v", customServers)
	runtime.LogInfof(a.ctx, "[App.startup] new loginServers: %+v", loginServers)
}

func (a *App) buildConfigPath() string {
	inDevMode := runtime.Environment(a.ctx).BuildType == "development"

	var basePath string
	if inDevMode {
		// in development mode, use the current working directory
		basePath, _ = os.Getwd()
	} else {
		// in production mode, use the executable path
		basePath, _ = os.Executable()
	}
	// get the directory of basePath
	dir := filepath.Dir(basePath)
	// construct the path to config.xml
	configPath := filepath.Join(dir, "config.xml")
	return configPath
}

// CheckForUpdate checks if there is a newer version of the application available.
// It retrieves the latest release tag from GitHub and compares it with the current version.
// If a newer version is available, it returns the latest tag string.
// If there's no update or an error occurs during the check process, it returns an empty string.
// Any errors encountered during the process are logged.
func (a *App) CheckForUpdate() string {
	currentVersion := a.appVersion

	latestTag, err := a.services.github.GetLatestReleaseTag()
	if err != nil {
		runtime.LogErrorf(a.ctx, "[App.GetGitHubLatestRelease] Failed to get latest release: %v", err)
		return ""
	}

	if latestTag == "" {
		runtime.LogInfo(a.ctx, "[App.GetGitHubLatestRelease] `latestTag` is empty string.")
		return ""
	}

	hasUpdate, err := a.hasUpdate(currentVersion, latestTag)
	if err != nil {
		runtime.LogErrorf(a.ctx, "[App.GetGitHubLatestRelease] Failed to compare versions: %v", err)
		return ""
	}

	if hasUpdate {
		runtime.LogInfof(a.ctx, "[App.GetGitHubLatestRelease] Update available: %s", latestTag)
		return latestTag
	} else {
		runtime.LogInfo(a.ctx, "[App.GetGitHubLatestRelease] No update available.")
		return ""
	}
}

// GetServers returns the list of servers
func (a *App) GetLoginServers() []LoginServer {
	runtime.LogInfof(a.ctx, "[App.GetLoginServers] loginServers: %+v", loginServers)
	return loginServers
}

// StopCapture stops the ongoing packet capturing process.
// It checks if there is an active packet capture service, stops it if it exists,
// sets the capture flag to false, and cleans up the service reference.
//
// Returns:
//   - bool: Always returns true, indicating the operation completed (whether or not
//     there was an actual service to stop).
func (a *App) StopCapture() bool {
	fmt.Println("[StopCapture] entering ...")

	if a.packetCaptureService != nil {
		a.packetCaptureService.StopCapture()
		a.isCapturing = false
		a.packetCaptureService = nil
		return true
	}

	fmt.Println("[StopCapture] No capture service to stop.")
	return true
}

// StartCapture initiates a packet capture session targeting the specified Ragnarok Online server.
// It first checks if a capture session is already running and stops it if necessary. Then, it constructs
// a network filter based on the provided targetServer name, matching it against the known loginServers.
// If a matching server is found, it starts capturing packets on all interfaces using the constructed filter.
// The function listens for packets on the capture channel, and upon receiving a packet that matches the
// expected pattern, it parses the payload into a list of CharacterServerInfo objects, stops the capture,
// and returns the list. If no matching server is found or the context is done, it returns nil.
//
// Parameters:
//   - targetServer: The name of the server to capture packets from.
//
// Returns:
//   - []CharacterServerInfo: A slice containing parsed character server information, or nil if no data is captured.
func (a *App) StartCapture(targetServerName string) []CharacterServerInfo {
	runtime.LogInfof(a.ctx, "[App.StartCapture] entering with targetServer: %s ...", targetServerName)

	if a.isCapturing || a.packetCaptureService != nil {
		runtime.LogWarningf(a.ctx, "[App.StartCapture] Already capturing, stop the previous capture.")

		// stop the running packet capture service if it exists
		a.packetCaptureService.StopCapture()

		// reset isCapturing flag and clean up packetCaptureService reference
		a.isCapturing = false
		a.packetCaptureService = nil
	}

	// find the target server in loginServers based on targetServerName
	var targetServer *LoginServer
	for _, server := range loginServers {
		if server.Name == targetServerName {
			targetServer = &server
			runtime.LogInfof(a.ctx, "[App.StartCapture] confirm target server: %s", targetServer.Name)
			break
		}
	}

	// if targetServer is nil, it means no matching server found
	if targetServer == nil {
		// TODO: add error handling, show a warning dialog to user
		runtime.LogWarningf(a.ctx, "[App.StartCapture] No matching server found for server name: %s", targetServerName)
		return nil
	}

	// construct the net filter for packet capture service by targetServer
	// filter := fmt.Sprintf("tcp and net %s and port %d", targetServer.IP, targetServer.Port)
	// runtime.LogInfof(a.ctx, "[App.StartCapture] build filter success: %s", filter)
	pattern := targetServer.Pattern

	// create a new packet capture service with the target server's IP and port
	packetCaptureService := network.NewPacketCaptureService(targetServer.IP, targetServer.Port)
	ctx := packetCaptureService.GetContext()
	channel := packetCaptureService.GetConnectionCloseNotifyChannel()

	// memorize the packetCaptureService and turn on isCapturing flag
	a.packetCaptureService = packetCaptureService
	a.isCapturing = true

	// start the packet capture service
	packetCaptureService.StartCaptureAllInterfaces()

	for {
		select {
		case connection := <-channel:
			// handle the connection close notification

			sortedIncomingData := connection.GetIncomingDataSortedByLength()

			for _, data := range sortedIncomingData {
				charServerInfoList := ragnarok.ParsePayloadToCharacterServerInfo(data, pattern)
				runtime.LogInfof(a.ctx, "[App.StartCapture] charServerInfoList: %+v", charServerInfoList)

				if charServerInfoList != nil {
					// stop the packet capture service
					packetCaptureService.StopCapture()
					// return the charServerInfoList
					return charServerInfoList
				}
			}

		case <-ctx.Done():
			// handle context done signal
			return nil
		}
	}

}

func (a *App) OpenGitHub() {
	runtime.BrowserOpenURL(a.ctx, "https://github.com/SDxBacon/RagnarokOnlinePlayerMonitor")
}

func (a *App) OpenAuthorPage() {
	runtime.BrowserOpenURL(a.ctx, "https://www.linkedin.com/in/renweiluo/")
}

// GetAppVersion returns the current version of the application, which is value of field `info.productVersion` in wails.json
func (a *App) GetAppVersion() string {
	return a.appVersion
}
