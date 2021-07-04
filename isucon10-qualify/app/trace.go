package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/trace"
)

func init() {
	http.HandleFunc("/traceStart", traceStart)
	http.HandleFunc("/traceStop", traceStop)
}

func traceStart(w http.ResponseWriter, r *http.Request) {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	err = trace.Start(f)
	if err != nil {
		panic(err)
	}
	w.Write([]byte("TrancStart"))
	fmt.Println("StartTrancs")
}

func traceStop(w http.ResponseWriter, r *http.Request) {
	trace.Stop()
	w.Write([]byte("TrancStop"))
	fmt.Println("StopTrancs")
}
