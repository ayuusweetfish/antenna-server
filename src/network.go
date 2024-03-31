package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func signUpHandler(w http.ResponseWriter, r *http.Request) {
	s := r.PostFormValue("handle")
	if s == "" {
		panic("> <")
		// panic(fmt.Errorf("> <"))
	}
	fmt.Fprintln(w, "hello "+s)
}

func avatarHandler(w http.ResponseWriter, r *http.Request) {
	handle := r.PathValue("profile_id")
	fmt.Fprintln(w, "avatar "+handle)
}

func roomCreateHandler(w http.ResponseWriter, r *http.Request) {
	room := Room{
		Title:       r.PostFormValue("title"),
		Tags:        r.PostFormValue("tags"),
		Description: r.PostFormValue("description"),
	}
	if err := room.Save(); err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%v\n", room)
}

func roomGetHandler(w http.ResponseWriter, r *http.Request) {
	room := Room{
		Id: r.PathValue("room_id"),
	}
	if err := room.Load(); err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%v\n", room)
}

// A handler that captures panics and return the error message as 500
type errCaptureHandler struct {
	Handler http.Handler
}

func (h *errCaptureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if obj := recover(); obj != nil {
			if err, ok := obj.(error); ok {
				http.Error(w, err.Error(), 500)
			} else {
				message := fmt.Sprint(obj)
				http.Error(w, message, 500)
			}
		}
	}()
	h.Handler.ServeHTTP(w, r)
}

func ServerListen() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sign-up", signUpHandler)
	mux.HandleFunc("GET /avatar/{profile_id}", avatarHandler)

	mux.HandleFunc("POST /room/new", roomCreateHandler)
	mux.HandleFunc("GET /room/{room_id}", roomGetHandler)

	port := Config.Port
	log.Printf("Listening on http://localhost:%d/\n", port)
	if Config.Debug {
		log.Printf("Visit http://localhost:%d/test for testing\n", port)
	}
	server := &http.Server{
		Handler:      &errCaptureHandler{Handler: mux},
		Addr:         fmt.Sprintf("localhost:%d", port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print(err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Print(err)
	}
	log.Print("Shutting down")
}
