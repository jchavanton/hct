package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// Compile templates on start of the application
var templates = template.Must(template.ParseFiles("public/cmd.html"))

// Display the named template
func display(w http.ResponseWriter, page string, data interface{}) {
	templates.ExecuteTemplate(w, page+".html", data)
}

func execDockerCmd(w http.ResponseWriter, uuid string) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	fmt.Printf("client created...\n")
	cli.NegotiateAPIVersion(ctx)

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		panic(err)
	}

	containerId := ""
	containerName := ""
	for _, ctr := range containers {
		if strings.Contains(ctr.Image, "hct_client") {
			containerId = ctr.ID
			containerName = ctr.Image
			fmt.Printf("hct_client container running %s %s\n", containerName, containerId)
			break
		}
	}
	if containerId == "" {
		fmt.Printf("hct_client container not running\n")
		//	http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	xml := fmt.Sprintf("/xml/%s.xml", uuid)
	out := fmt.Sprintf("/output/%s.json", uuid)
	port := "7070"
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{"/git/voip_patrol/voip_patrol",
			"--port", port,
			"--conf", xml,
			"--output", out},
	}

	response, err := cli.ContainerExecCreate(ctx, containerId, execConfig)
	//response, err := cli.ContainerExecAttach(ctx, containerId, config)
	if err != nil {
		panic(err)
	}
	startConfig := types.ExecStartCheck{
		Detach: true,
		Tty:    false,
	}
	fmt.Printf("ContainerExecCreate [%s]\n", containerId)

	err = cli.ContainerExecStart(ctx, response.ID, startConfig)
	if err != nil {
		panic(err)
	}
	fmt.Printf("ContainerExecStart [%s]\n", containerId)

	execInspect, err := cli.ContainerExecInspect(ctx, response.ID)
	fmt.Printf("ContainerExecInspect pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
}

type Call struct {
	Ruri string `json:"r-uri"`
}

type Cmd struct {
	Call Call `json:"call"`
}

func createXmlFile(w http.ResponseWriter, uuid string, xml string) {
	// Create file
	dst, err := os.Create("/xml/" + uuid + ".xml")
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
	execDockerCmd(w, uuid)
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
