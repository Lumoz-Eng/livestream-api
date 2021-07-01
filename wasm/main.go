// +build js,wasm

package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"syscall/js"
	"time"
)

var interrupt = make(chan bool, 1)
var stillFetching = false

//this function will fetch image from the server in which the client is sharing his screen
//and this will be streamed to users who are viewing the shared-screen route
func loadImage() string {
	href := js.Global().Get("location").Get("href")
	u, err := url.Parse(href.String())
	if err != nil {
		log.Fatal(err)
	}

	u.Path = "/fetch-png"
	resp, err := http.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	png, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	stillFetching = false
	return base64.StdEncoding.EncodeToString(png) //we convert bytes into string with base64 encoding so that we can make this as data image source
}

//this is our main Webassembly function, we will communicate with the browser via wasm and js
func main() {
	document := js.Global().Get("document")             //this is getting the document object from the browser
	image := js.Global().Call("eval", "new Image()")    //creating image using wams js
	canvas := document.Call("getElementById", "canvas") //getting the canvas by its id
	ctx := canvas.Call("getContext", "2d")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go gracefulTerminateSystem()

	//here we will keep fetching the PNG bytes from the client server who is sharing his screen
	//then we draw it on the canvas in the browser via wasm
	for {
		select {
		case <-ticker.C:
			if !stillFetching {
				stillFetching = true
				image.Set("src", "data:image/png;base64,"+loadImage())
				ctx.Call("drawImage", image, 0, 0) //draw the fetch png image from the user sharing his screen
			}

		case <-interrupt:
			select {
			case <-time.After(time.Second):
			}
			return
		}
	}
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
