package main

import (
	"context"
	"encoding/json"
	"errors"
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

type SummaryReport struct {
	Calls int32
	TxPkt int32
	TxBytes int32
	RxPkt int32
	RxBytes int32
	Duration int32
}

type Call struct {
	Ruri string `json:"r-uri"`
	Repeat int `json:"repeat"`
}

type Cmd struct {
	Call Call `json:"call"`
}

type RtpTransfer struct {
	JitterAvg float32 `json:"jitter_avg"`
	JitterMax float32 `json:"jitter_max"`
	Pkt       int32   `json:"pkt"`
	Kbytes    int32   `json:"kbytes"`
	Loss      int32   `json:"loss"`
	Discard   int32   `json:"discard"`
	Mos       float32 `json:"mos_lq"`
}

type RtpStats struct {
	Rtt          int         `json:"rtt"`
	RemoteSocket string      `json:"remote_rtp_socket"`
	CodecName    string      `json:"codec_name"`
	CodecRate    string      `json:"codec_rate"`
	Tx           RtpTransfer `json:"Tx"`
	Rx           RtpTransfer `json:"Rx"`
}

type CallInfo struct {
	LocalUri      string `json:"local_uri"`
	RemoteUri     string `json:"remote_uri"`
	LocalContact  string `json:"local_contact"`
	RemoteContact string `json:"remote_contact"`
}
type TestReport struct {
	Label            string     `json:"label"`
	Start            string     `json:"start"`
	End              string     `json:"end"`
	Action           string     `json:"action"`
	From             string     `json:"from"`
	To               string     `json:"to"`
	Result           string     `json:"result"`
	ExpectedCode     int32      `json:"expected_cause_code"`
	CauseCode        int32      `json:"cause_code"`
	Reason           string     `json:"reason"`
	CallId           string     `json:"callid"`
	Transport        string     `json:"transport"`
	PeerSocket       string     `json:"peer_socket"`
	Duration         int32      `json:"duration"`
	ExpectedDuration int32      `json:"expected_duration"`
	MaxDuration      int32      `json:"max_duration"`
	HangupDuration   int32      `json:"hangup_duration"`
	CallInfo         CallInfo   `json:"call_info"`
	RtpStats         []RtpStats `json:"rtp_stats"`
}


// Compile templates on start of the application
var templates = template.Must(template.ParseFiles("public/cmd.html"))

// Display the named template
func display(w http.ResponseWriter, page string) {
	type Vars struct {
		LocalIp string
		VpServer string
	}
	data := Vars{os.Getenv("LOCAL_IP"), os.Getenv("VP_SERVER")} 
	err := templates.ExecuteTemplate(w, page+".html", data)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func cmdDockerExec(w http.ResponseWriter, uuid string, rtp_port int, sip_port int) {
	sport := fmt.Sprintf("%d", sip_port)
	rport := fmt.Sprintf("%d", rtp_port)
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx := context.Background()
	fmt.Printf("client created... [%s]\n", sport)
	cli.NegotiateAPIVersion(ctx)

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
	// port := "7070"
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{"/git/voip_patrol/voip_patrol", "--udp",  "--rtp-port", rport,
			"--port", sport,
			"--conf", xml,
			"--output", out},
	}

	response, err := cli.ContainerExecCreate(ctx, containerId, execConfig)
	//response, err := cli.ContainerExecAttach(ctx, containerId, config)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	startConfig := types.ExecStartCheck{
		Detach: true,
		Tty:    false,
	}
	fmt.Printf("ContainerExecCreate [%s]\n", containerId)

	err = cli.ContainerExecStart(ctx, response.ID, startConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Printf("ContainerExecStart [%s]\n", containerId)

	execInspect, err := cli.ContainerExecInspect(ctx, response.ID)
	fmt.Printf("ContainerExecInspect pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
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
	} else {
		fmt.Printf("empty request URI\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
	}
	uuid := uuid.NewString()
	createCall(w, uuid+"-0", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 10000, 15060)
	createCall(w, uuid+"-1", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 10200, 15061)
	createCall(w, uuid+"-2", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 10400, 15062)
	createCall(w, uuid+"-3", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 10600, 15063)
	createCall(w, uuid+"-4", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 10800, 15064)
	createCall(w, uuid+"-5", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 11000, 15065)
	createCall(w, uuid+"-6", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 11200, 15066)
	createCall(w, uuid+"-7", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 11400, 15067)
	createCall(w, uuid+"-8", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 11600, 15068)
	createCall(w, uuid+"-9", cmd.Call.Ruri, cmd.Call.Ruri, cmd.Call.Repeat, 11800, 15069)
	w.WriteHeader(200)
	w.Write([]byte("<html><a href=\"http://"+os.Getenv("LOCAL_IP")+":8080/res?"+uuid+"\">check report for "+uuid+"</a></html>"))
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
		display(w, "cmd")
	case "POST":
		cmdExec(w, r)
	}
}

func processResultFile(w http.ResponseWriter, r *http.Request, fn string, report *SummaryReport) {
	file, err := os.Open("/output/"+fn)
	if err != nil {
		fmt.Printf("error opening result file [%s]\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		// fmt.Println(scanner.Text())
		var testReport TestReport
		b := []byte(scanner.Text())
		err := json.Unmarshal(b, &testReport)
		if err != nil {
			fmt.Printf("invalid test report[%s][%s]\n", scanner.Text(), err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if testReport.Action == "call" {
			report.Calls += 1
			report.TxPkt += testReport.RtpStats[0].Tx.Pkt
			report.TxBytes += testReport.RtpStats[0].Tx.Kbytes
			report.RxPkt += testReport.RtpStats[0].Rx.Pkt
			report.RxBytes += testReport.RtpStats[0].Rx.Kbytes
			report.Duration += testReport.Duration
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("error opening reading result file [%s]\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
	}
}

func resHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("id")
		fmt.Println("id =>", uuid)
	entries, err := os.ReadDir("/output")
	if err != nil {
		fmt.Printf("error opening result file [%s]\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
    	}
	var report SummaryReport
    	for _, e := range entries {
		s := e.Name()
		// fmt.Println(s)
		if  s[len(s)-5:] == ".json" && strings.Contains(s, uuid) {
			processResultFile(w, r, s, &report)
    		}
    	}

	reportJson, _ := json.Marshal(report)
	fmt.Fprintf(w, string(reportJson))
	fmt.Println(string(reportJson))
	ua := r.Header.Get("User-Agent")
	m := "res"
	fmt.Printf("[%s] %s...\n", ua, m)
}

func createCall(w http.ResponseWriter, uuid string, from string, to string, repeat int, rtp_port int, sip_port int) {
	xml := fmt.Sprintf(`
<config>
  <actions>
    <action type="call" label="%s"
            transport="udp"
            expected_cause_code="200"
            caller="hct_controller@noreply.com"
	    callee="%s:%d"
	    to_uri="%s"
            repeat="%d"
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
	`, uuid, from, sip_port, to, repeat)
	fmt.Printf("%s\n", xml)
	createXmlFile(w, uuid, xml)

	cmdDockerExec(w, uuid, rtp_port, sip_port)
	time.Sleep(100 * time.Millisecond)
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
	http.HandleFunc("/res", resHandler)
	// http.HandleFunc("/download", downloadHandler)

	fmt.Printf("version[%s] Listen on port %d\n", version, port)
	e = http.ListenAndServe(":"+os.Args[1], nil)
	if e != nil {
		fmt.Printf("ListenAndServe: %s\n", e)
	}
}
