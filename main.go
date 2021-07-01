package main

import (
	"bytes"
	"fmt"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/kbinani/screenshot" //--> this the screenshot package that we use to share our screen
)

var interrupt = make(chan bool, 1)
var bytesToSend = make([]byte, 0) //this is the current PNG screenshot to send to users.

func main() {
	addr := ":8080"
	prefix := "/"
	root := "./wasm/"

	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		log.Fatalln(err)
	}

	currentTime := time.Now()
	fmt.Println("Share Screen server is starte...", currentTime.Format("Mon 02 Jan 2006 03:04pm"))
	log.Printf("serving %s as %s on %s", root, prefix, addr)
	http.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(root))))

	//routes
	http.HandleFunc("/start-sharing", takeScreenShot)
	http.HandleFunc("/shared-screen", fetchScreenShot)
	http.HandleFunc("/stop-sharing", stopSharing)
	http.HandleFunc("/fetch-png", fetchPNG)

	mux := http.DefaultServeMux.ServeHTTP
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr + " " + r.Method + " " + r.URL.String())
		mux(w, r)
	})

	err = http.ListenAndServe(addr, logger)
	if err != nil {
		log.Fatalln(err)
	}

	go gracefulTerminateSystem()
}

//this is to take screen shot of the user's selected screen to share
//we will be taking screen shots in every 500 millisecond until stop sharing is
//triggered by the user
func takeScreenShot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Sharing Screen Started")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	buf := new(bytes.Buffer)
	//screenRegion := image.Rext(0,0,800,600) //do this if you want to share portion of your screen only

	for {
		select {
		case <-ticker.C:
			//img, err := screenshot.CaptureRect(screenRegion) //this is to capture the specific region of your screen

			//for this tutorial we will capture the entire screen of primary display monitor
			img, err := screenshot.CaptureDisplay(0) //0 is for the primary screen, if you have 2 monitors pass in 1 to capture the second monitor or screen
			if err != nil {
				panic(err)
			}

			png.Encode(buf, img)
			bytesToSend = buf.Bytes()
			buf.Reset()
		case <-interrupt:
			log.Println("Terminating stream...")
			select {
			case <-time.After(time.Second):
			}
			return
		}
	}

}

func fetchScreenShot(w http.ResponseWriter, r *http.Request) {
	filename := "./wasm/shared-screen.html" //we will server this html file to the user to start sharing screen
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprint(w, err)
	}
	fmt.Fprintf(w, string(body))
}

func stopSharing(w http.ResponseWriter, r *http.Request) {
	interrupt <- true
	fmt.Fprintf(w, "Share Screen has been stopped")
}

//this will send the current screenshot PNG to the user who wants to view the shared screen
func fetchPNG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(bytesToSend)))
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, pre-check=0") //we dont want to cache the screenshot image here
	if _, err := w.Write(bytesToSend); err != nil {
		log.Println("Unable to write image")
	}
	log.Println("PNG sent to user")
}

func gracefulTerminateSystem() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		interrupt <- true
		fmt.Println("Ctrl+C was pressed in terminal")
		os.Exit(0)
	}()

}
