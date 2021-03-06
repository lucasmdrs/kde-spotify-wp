package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

var (
	homeDir = os.Getenv("HOME")

	localPath  = fmt.Sprintf("%s/.local/share/wallpapers/", homeDir)
	defaultArt = fmt.Sprintf("file://%sdefault", localPath)
	prevImg    = defaultArt
	currentImg = defaultArt
)

const (
	dbusListMethod = "org.freedesktop.DBus.ListNames"

	spotifyDestination      = "org.mpris.MediaPlayer2.spotify"
	spotifyPath             = "/org/mpris/MediaPlayer2"
	spotifyMetadataProperty = "org.mpris.MediaPlayer2.Player.Metadata"
	spotifyMetaURL          = "xesam:url"
	spotifyEmbedObjectURL   = "https://open.spotify.com/oembed?url=%s"

	plasmaDestination    = "org.kde.plasmashell"
	plasmaPath           = "/PlasmaShell"
	plasmaEvaluateMethod = "org.kde.PlasmaShell.evaluateScript"

	plasmaScriptTemplate = `var allDesktops = desktops();
	for (i=0;i<allDesktops.length;i++) {
		d = allDesktops[i];
		d.wallpaperPlugin = "org.kde.image";
		d.currentConfigGroup = Array("Wallpaper", "org.kde.image", "General");
		d.writeConfig("Image", "%s");
		d.writeConfig("FillMode", 3);
	}`
)

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to SessionBus:", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	for {
		<-time.After(5 * time.Second)
		if hasSpotify(conn) {
			currentImg = getArtURL(conn)
		}
		if currentImg != prevImg {
			fmt.Println("Changing to", currentImg)
			changeBackgroud(conn, currentImg)
		}
	}

}

func hasSpotify(conn *dbus.Conn) bool {
	var s []string
	if err := conn.BusObject().Call(dbusListMethod, 0).Store(&s); err != nil {
		return false
	}

	for _, v := range s {
		if v == spotifyDestination {
			return true
		}
	}
	return false
}

func getArtURL(conn *dbus.Conn) string {
	data, err := conn.Object(spotifyDestination, spotifyPath).GetProperty(spotifyMetadataProperty)
	if err != nil {
		return defaultArt
	}
	resp, err := http.Get(fmt.Sprintf(spotifyEmbedObjectURL, data.Value().(map[string]dbus.Variant)[spotifyMetaURL].Value().(string)))
	if err != nil {
		return defaultArt
	}

	obj := struct {
		URL string `json:"thumbnail_url"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&obj)
	if err != nil {
		return defaultArt
	}

	splitedURL := strings.Split(obj.URL, "/")
	filepath := localPath + splitedURL[len(splitedURL)-1]
	_, err = os.Stat(filepath)
	if err == nil {
		return fmt.Sprintf("file://%s", filepath)
	}

	return downloadFile(obj.URL, filepath)
}

func downloadFile(url, filepath string) string {
	resp, err := http.Get(url)
	if err != nil {
		return defaultArt
	}
	defer resp.Body.Close()

	f, err := os.Create(filepath)
	if err != nil {
		return defaultArt
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	r := bufio.NewReader(resp.Body)

	_, err = io.Copy(w, r)
	if err != nil {
		return defaultArt
	}

	return fmt.Sprintf("file://%s", filepath)

}

func changeBackgroud(conn *dbus.Conn, url string) {
	call := conn.Object(plasmaDestination, plasmaPath).Call(plasmaEvaluateMethod, 0, fmt.Sprintf(plasmaScriptTemplate, url))
	if call.Err != nil {
		fmt.Fprintln(os.Stderr, "Failed to change background", call.Err.Error())
	}
	prevImg = url
}
