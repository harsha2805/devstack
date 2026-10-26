// Command uipreview serves the dashboard without the tray or window, for
// checking it in a browser or taking screenshots:
//
//	DEVSTACK_APPDATA=<temp dir> go run ./tools/uipreview -exe dist\devstack.exe
//
// It prints the dashboard URL (with its token) and serves until killed.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/harsha2805/devstack/internal/ui"
)

func main() {
	exe := flag.String("exe", "", "devstack binary used to start the daemon")
	flag.Parse()
	srv := &ui.Server{Version: "preview", Exe: *exe}
	if _, err := srv.Listen(); err != nil {
		log.Fatal(err)
	}
	fmt.Println(srv.URL())
	select {}
}
