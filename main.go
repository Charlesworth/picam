package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/charlesworth/picam/camera"
	"github.com/gorilla/handlers"
	"github.com/julienschmidt/httprouter"
)

func pictureHandler(picam camera.Picam) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		pic, err := picam.Capture()
		if err != nil {
			log.Println("unable to capture picture:", err)
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err.Error()), 400)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(pic)))

		if _, err := w.Write(pic); err != nil {
			log.Println("unable to write image")
		}
	}
}

func startRecordingHandler(picam camera.Picam) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		err := picam.StartRecording()
		if err != nil {
			log.Println("unable to start recording:", err)
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err.Error()), 400)
			return
		}
		w.WriteHeader(200)
	}
}

func retrieveVideoHandler(picam camera.Picam) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		filename, err := picam.StopRecording()
		if err != nil {
			log.Println("unable to stop recording:", err)
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err.Error()), 400)
			return
		}

		vid, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatal("unable to read video output:", err)
		}

		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", strconv.Itoa(len(vid)))

		if _, err := w.Write(vid); err != nil {
			log.Println("unable to write video")
		}
	}
}

func main() {
	port := flag.Int("p", 8000, "port to bind to")
	flag.Parse()

	log.Printf("starting server on port :%v\n", *port)
	picam := camera.NewPicam()

	router := httprouter.New()
	router.GET("/pic.jpg", pictureHandler(picam))
	router.POST("/video/start", startRecordingHandler(picam))
	router.GET("/video/stop", retrieveVideoHandler(picam))

	loggedRouter := handlers.LoggingHandler(os.Stdout, router)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", *port), loggedRouter))
}
