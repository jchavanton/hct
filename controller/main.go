package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"text/template"
	"strconv"
)

// Compile templates on start of the application
var templates = template.Must(template.ParseFiles("public/cmd.html"))

// Display the named template
func display(w http.ResponseWriter, page string, data interface{}) {
	templates.ExecuteTemplate(w, page+".html", data)
}

func cmdExec(w http.ResponseWriter, r *http.Request) {
	// Maximum upload of 16 MB files
	r.ParseMultipartForm(16 << 20)

	var fileName string
	for k, v := range r.MultipartForm.File{
		fileName = k
		fmt.Println(k)
		fmt.Println(v)
		break
	}

	// Get handler for filename, size and headers
	file, handler, err := r.FormFile(fileName)
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}

	defer file.Close()
	// fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	// fmt.Printf("File Size: %+v\n", handler.Size)
	// fmt.Printf("MIME Header: %+v\n", handler.Header)

	// Create file
	dst, err := os.Create("cmd/" + handler.Filename)
	defer dst.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy the uploaded file to the created file on the filesystem
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Successfully Uploaded File [%s] size[%d]\n", handler.Filename, handler.Size)
	fmt.Fprintf(w, "Successfully Uploaded File [%s] size[%d]\n", handler.Filename, handler.Size)
}

func checkFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
}

func cmdHandler(w http.ResponseWriter, r *http.Request) {
	ua := r.Header.Get("User-Agent")
	m := "cmd"
	fmt.Printf("[%s] %s...\n", ua, m)
	switch r.Method {
	case "GET":
		display(w, "cmd", nil)
	case "POST":
		cmdExec(w, r)
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	ua := r.Header.Get("User-Agent")
	m := "download"
	fn := r.URL.Query().Get("fn")
	if fn == "" {
		fmt.Printf("[%s]%s: missing fn param\n", ua, m)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	filePath := "cmd/" + fn
	isFileExist := checkFileExists(filePath)
	if !isFileExist {
		fmt.Printf("[%s]%s[%s] file not found\n", ua, m, fn)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("[%s]%s[%s] error reading file\n", ua, m, fn)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("[%s]%s[%s]\n", ua, m, fn)
	w.Header().Set("Content-Disposition", "attachment; filename="+fn)
	// w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(fileBytes)
	return
}

func main() {
        version := "0.0.0"
	if len(os.Args) < 4 {
		fmt.Printf("Missing argument %d\n", len(os.Args))
		return
	}
	port, e := strconv.Atoi(os.Args[1])
	if e != nil {
		fmt.Printf("Invalid argument port %s\n", os.Args[1])
		return
	}
	cert := os.Args[2]
	key := os.Args[3]
	fmt.Printf("cert[%s] key[%s]\n", cert, key)

	// Upload route
	http.HandleFunc("/cmd", cmdHandler)
	http.HandleFunc("/download", downloadHandler)

	fmt.Printf("version[%s] Listen on port %d\n", version, port)
	e = http.ListenAndServe(":"+os.Args[1], nil)
	if e != nil {
		fmt.Printf("ListenAndServeTLS: ", e)
	}
}
