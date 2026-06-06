package main

import (
	"fmt"
	"os"
)

func SaveData1(path string, data []byte) error {
	fp, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer fp.Close()
	_, err = fp.Write(data) //_ here is used for error handling, we are ignoring the path which is string
	return err
}

func main() {
	err := SaveData1("tes.txt", []byte("Hello, World!"))
	if err != nil {
		fmt.Println("failed to load the database ")
		return
	}
	fmt.Println("Data saved successfully.")
}
