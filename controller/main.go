package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"text/template"

	"github.com/google/uuid"
)

// Compile templates on start of the application
var templates = template.Must(template.ParseFiles("public/cmd.html"))

// Display the named template
func display(w http.ResponseWriter, page string, data interface{}) {
	templates.ExecuteTemplate(w, page+".html", data)
}

type Call struct {
	Ruri string `json:"r-uri"`
}

type Cmd struct {
	Call Call `json:"call"`
}

func createXmlFile(w http.ResponseWriter, uuid string, xml string) {
	// Create file
	dst, err := os.Create("/xml/"+uuid+".xml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile("/xml/"+uuid+".xml", []byte(xml), 0666); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer dst.Close()
}

func cmdExec(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s := r.FormValue("cmd")

	cmd := new(Cmd)
	b := []byte(s)

	err := json.Unmarshal(b, cmd)
	if err != nil {
		fmt.Printf("invalid command [%s][%s]\n", cmd, err)
	}
	if cmd.Call.Ruri != "" {
		fmt.Printf("call[%s]\n", cmd.Call.Ruri)
	}
	uuid := uuid.NewString()
	createCall(w, uuid, cmd.Call.Ruri, cmd.Call.Ruri)
	w.WriteHeader(200)
	w.Write([]byte(uuid))
	return
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

func createCall(w http.ResponseWriter, uuid string, from string, to string) {
	xml := fmt.Sprintf(`
<config>
  <actions>
    <action type="call" label="%s"
            transport="udp"
            expected_cause_code="200"
            caller="hct_controller@noreply.com"
            callee="%s"
            to_uri="%s"
            max_duration="20" hangup="16"
            username="VP_ENV_USERNAME"
            password="VP_ENV_PASSWORD"
            rtp_stats="true"
    >
    <x-header name="X-Foo" value="Bar"/>
   </action>
  <action type="wait" complete="true"/>
 </actions>
</config>

	`, uuid, from, to)
	fmt.Printf("%s\n", xml)
	createXmlFile(w, uuid, xml)
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
	// http.HandleFunc("/download", downloadHandler)

	fmt.Printf("version[%s] Listen on port %d\n", version, port)
	e = http.ListenAndServe(":"+os.Args[1], nil)
	if e != nil {
		fmt.Printf("ListenAndServe: %s\n", e)
	}
}
