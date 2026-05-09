package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/opentalon/opentalon/pkg/plugin"
)

func main() {
	log.SetOutput(os.Stderr)
	log.Println("planner-plugin: process starting")

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/opentalon/opentalon" {
				log.Printf("planner-plugin: opentalon SDK %s", dep.Version)
			}
		}
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("planner-plugin: PANIC: %v\n%s", r, debug.Stack())
			fmt.Fprintf(os.Stderr, "planner-plugin: PANIC: %v\n", r)
			os.Exit(1)
		}
	}()

	log.Println("planner-plugin: calling plugin.Serve")
	if err := plugin.Serve(NewHandler()); err != nil {
		log.Fatalf("planner-plugin: plugin.Serve error: %v", err)
	}
}
