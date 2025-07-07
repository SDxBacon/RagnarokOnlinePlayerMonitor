package main

import (
	"bytes"
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

type LoginServer = config.LoginServer

type CharacterServerInfo = ragnarok.CharacterServerInfo

var loginServers []LoginServer = []LoginServer{
	{Name: "Taiwan", IP: "219.84.200.54", Port: 6900, Pattern: []byte{0xc0, 0xa8}},
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

func (a *App) StartCapture(targetServer string) []CharacterServerInfo {
	runtime.LogInfof(a.ctx, "[App.StartCapture] entering with targetServer: %s ...", targetServer)

	if a.isCapturing || a.packetCaptureService != nil {
		runtime.LogWarningf(a.ctx, "[App.StartCapture] Already capturing, stop the previous capture.")

		prevPacketCaptureService := a.packetCaptureService
		prevPacketCaptureService.StopCapture() // stop the previous capture

		// reset isCapturing flag and clean up packetCaptureService reference
		a.isCapturing = false
		a.packetCaptureService = nil
	}

	// construct the net filter for packet capture service by targetServer
	var filter string
	for _, server := range loginServers {
		if server.Name == targetServer {
			filter = fmt.Sprintf("tcp and net %s and port %d", server.IP, server.Port)
			runtime.LogInfof(a.ctx, "[App.StartCapture] build filter success: %s", filter)
			break
		}
	}
	// if filter is empty, it means no matching server found
	if filter == "" {
		runtime.LogWarningf(a.ctx, "[App.StartCapture] No matching server found for targetServer: %s", targetServer)
		return nil
	}

	packetCaptureService := network.NewPacketCaptureService(filter)
	ctx := packetCaptureService.GetContext()
	channel := packetCaptureService.GetPacketChannel()

	// memorize the packetCaptureService and turn on isCapturing flag
	a.packetCaptureService = packetCaptureService
	a.isCapturing = true

	// start the packet capture service
	packetCaptureService.StartCaptureAllInterfaces()

	for {
		select {
		case payload := <-channel:
			// receive packet from channel
			pattern := loginServers[0].Pattern
			// check if the payload is started with `pattern`
			if !bytes.Equal(payload[:2], pattern) {
				continue
			}

			charServerInfoList := ragnarok.ParsePayloadToCharacterServerInfo(payload, pattern)

			runtime.LogInfof(a.ctx, "[App.StartCapture] charServerInfoList: %+v", charServerInfoList)

			// stop the packet capture service
			packetCaptureService.StopCapture()
			// return the charServerInfoList
			return charServerInfoList
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
