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
	TxKbytes int32
	TxLost int32
	TxJitterMax float32
	RxPkt int32
	RxKbytes int32
	RxLost int32
	RxJitterMax float32
	Duration int32
	AvgDuration float32
	Failed int32
	Connected int32
}

type Call struct {
	Ruri string `json:"destination"`
	Count int `json:"count"`
	Username string `json:"username"`
	Password string `json:"password"`
	Duration int `json:"duration"`
}

type CallParams struct {
	Ruri string
	Repeat int
	Username string
	Password string
	Duration int
	PortRtp int
	PortSip int
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
		http.Error(w, "hct_client container not running", http.StatusInternalServerError)
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

func cmdMakeCalls(w http.ResponseWriter, r *http.Request, cmd *Cmd, uuid string) {
	port_rtp := 10000
	port_sip := 15060
	idx := 0
	calls := cmd.Call.Count
	for calls > 0 {
		n := fmt.Sprintf("%s-%d", uuid, idx)
		if calls < 50 {
			params := CallParams{cmd.Call.Ruri,calls-1,cmd.Call.Username,cmd.Call.Password,cmd.Call.Duration,port_rtp,port_sip}
			cmdCreateCall(w, n, params)
			calls = 0
		} else {
			params := CallParams{cmd.Call.Ruri,49,cmd.Call.Username,cmd.Call.Password,cmd.Call.Duration,port_rtp,port_sip}
			cmdCreateCall(w, n, params)
			calls -= 50
		}
		port_rtp += 200;
		port_sip += 1;
		idx += 1;
	}
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
		http.Error(w, "empty request URI", http.StatusInternalServerError)
		return;
	}
	uuid := uuid.NewString()

	if cmd.Call.Count > 1000 {
		fmt.Printf("too many calls requested [%d/%d]\n", cmd.Call.Count, 1000)
		http.Error(w, "too many calls requested", http.StatusInternalServerError)
		return;
	}

	w.WriteHeader(200)
	w.Write([]byte("<html><a href=\"http://"+os.Getenv("LOCAL_IP")+":8080/res?id="+uuid+"\">check report for "+uuid+"</a></html>"))
	go cmdMakeCalls(w, r, cmd, uuid)
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

func resProcessResultFile(w http.ResponseWriter, r *http.Request, fn string, report *SummaryReport) {
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
		fmt.Println(scanner.Text())
		var testReport TestReport
		b := []byte(scanner.Text())
		err := json.Unmarshal(b, &testReport)
		if err != nil {
			fmt.Printf("invalid test report[%s][%s]\n", scanner.Text(), err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if testReport.Action == "call" {
			report.Calls += 1
			if len(testReport.RtpStats) > 0 {
				report.TxPkt += testReport.RtpStats[0].Tx.Pkt
				report.TxKbytes += testReport.RtpStats[0].Tx.Kbytes
				report.TxLost += testReport.RtpStats[0].Tx.Loss
				if report.TxJitterMax < testReport.RtpStats[0].Tx.JitterMax {
					report.TxJitterMax = testReport.RtpStats[0].Tx.JitterMax
				}
				report.RxPkt += testReport.RtpStats[0].Rx.Pkt
				report.RxKbytes += testReport.RtpStats[0].Rx.Kbytes
				report.RxLost += testReport.RtpStats[0].Rx.Loss
				if report.RxJitterMax < testReport.RtpStats[0].Rx.JitterMax {
					report.RxJitterMax = testReport.RtpStats[0].Rx.JitterMax
				}
			}
			if testReport.CauseCode >= 300 {
				report.Failed += 1
			} else if testReport.CauseCode >= 200 {
				report.Connected += 1
				report.Duration += testReport.Duration
				report.AvgDuration = float32(report.Duration/report.Calls)
			}
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
	if uuid == "" {
		fmt.Printf("missing id parameter\n")
		http.Error(w, "missing id parameter", http.StatusInternalServerError)
		return;
	}
	if err != nil {
		fmt.Printf("error opening result file [%s]\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
    	}
	var report SummaryReport
    	for _, e := range entries {
		s := e.Name()
		if  s[len(s)-5:] == ".json" && strings.Contains(s, uuid) {
			fmt.Println(s)
			resProcessResultFile(w, r, s, &report)
    		}
    	}

	reportJson, _ := json.Marshal(report)
	fmt.Fprintf(w, string(reportJson))
	fmt.Println(string(reportJson))
	ua := r.Header.Get("User-Agent")
	m := "res"
	fmt.Printf("[%s] %s...\n", ua, m)
}

func cmdCreateCall(w http.ResponseWriter, uuid string, p CallParams) {
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
	    username="%s"
	    password="%s"
            max_duration="%d" hangup="%d"
            username="VP_ENV_USERNAME"
            password="VP_ENV_PASSWORD"
            rtp_stats="true"
    >
    <x-header name="X-Foo" value="Bar"/>
   </action>
  <action type="wait" complete="true"/>
 </actions>
</config>
	`, uuid, p.Ruri, 5555, p.Ruri, p.Repeat, p.Username, p.Password, p.Duration+2, p.Duration)
	fmt.Printf("%s\n", xml)
	createXmlFile(w, uuid, xml)

	cmdDockerExec(w, uuid, p.PortRtp, p.PortSip)
	time.Sleep(1000 * time.Millisecond)
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
