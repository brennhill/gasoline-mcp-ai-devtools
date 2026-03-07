package main

import (
	"github.com/example/web/handlers"
	"github.com/example/web/routes"
)

func main() {
	r := routes.Setup()
	handlers.Register(r)
}
