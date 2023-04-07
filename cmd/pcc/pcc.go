package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/dslipak/pdf"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("Expected 1 argument: source PDF file")
	}
	file := os.Args[1]

	if err := run(file); err != nil {
		log.Fatalln(err)
	}
}

func run(filename string) error {
	rdr, err := pdf.Open(filename)
	if err != nil {
		return err
	}

	prdr, err := rdr.GetPlainText()
	if err != nil {
		return err
	}

	contents, err := ioutil.ReadAll(prdr)
	if err != nil {
		return err
	}

	fmt.Printf("Number of pages: %d\n", rdr.NumPage())
	for i := 0; i < rdr.NumPage(); i++ {
		page := rdr.Page(i + 1)
		for _, name := range page.Fonts() {
			fmt.Printf("Font: %s\n", name)
		}
		content := page.Content()
		for _, text := range content.Text {
			fmt.Printf("Text: %s\n", text.S)
		}
	}

	fmt.Println(string(contents))
	return nil
}
