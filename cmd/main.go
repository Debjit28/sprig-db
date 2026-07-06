package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Debjit28/sprig-db/api"
	"github.com/Debjit28/sprig-db/sprig"
	"github.com/labstack/echo/v4"
)

func main() {
	var a any
	a = false
	b := false
	fmt.Println(a == b)
	db, err := sprig.New()
	if err != nil {
		log.Fatal(err)
	}
	server := api.NewServer(db)

	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	e.HideBanner = true
	e.POST("/api/:collname", server.HandlePostInsert)
	e.GET("/api/:collname", server.HandleGetQuery)
	log.Fatal(e.Start(":7777"))
}
