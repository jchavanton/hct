package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
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
	amqp "github.com/rabbitmq/amqp091-go"
)

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
	Uuid string
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

type SipLatency struct {
	Invite100Ms int32 `json: "invite100Ms"`
	Invite18xMs int32 `json: "invite18xMs"`
	Invite200Ms int32 `json: "invite200Ms"`
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
	SipLatency       SipLatency `json:"sip_latency"`
	RtpStats         []RtpStats `json:"rtp_stats"`
}

type ReportRtpPkt struct {
	Pkt int32
	Kbytes int32
	Lost int32
	JitterMax float32
}

type ReportRtp struct {
	RttAvg int32
	Tx ReportRtpPkt
	Rx ReportRtpPkt
}

type Stat struct {
	Min int32 `json:"min_ms"`
	Max int32 `json:"max_ms"`
	Average float32 `json:"avg_ms"`
	Stdev float32 `json:"std_ms"`   // last standard deviation
	m2 float64 // sum of squares, used for recursive variance calculation
	Count int32 `json:"count"`
}

type ReportSip struct {
	Invite100 Stat `json:"invite100"`
	Invite18x Stat `json:"invite18x"`
	Invite200 Stat `json:"invite200"`
}

type Report struct {
	Calls int32
	Duration int32
	AvgDuration float32
	Failed int32
	Connected int32
	Sip ReportSip
	Rtp ReportRtp
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

func rmqPublish(queue string, report string) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queue, // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing {
			ContentType: "text/plain",
			Body:        []byte(report),
		})
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	fmt.Printf(" [x] Sent %s\n", report)
}

func rmqSubscribe() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"commands", // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}

	fmt.Printf(" [*] Waiting for messages.\n")
	var forever chan struct{}
	go func() {
		for d := range msgs {
			fmt.Printf("command message received: %s\n", d.Body)
			cmd, err := cmdCreate(string(d.Body[:]))
			if err != nil {
				fmt.Printf("cmdCreate: message received error:", err)
			}
			go func (cmd *Cmd) {
				err := cmdMakeCalls(cmd)
				if err != nil {
					fmt.Printf("cmdMakeCalls error: %s\n", err)
				}
			} (cmd)
		}
	} ()
	<-forever
}

func cmdDockerExec(uuid string, rtp_port int, sip_port int, expected_duration int) (error) {
	sport := fmt.Sprintf("%d", sip_port)
	rport := fmt.Sprintf("%d", rtp_port)
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	ctx := context.Background()
	fmt.Printf("client created... [%s]\n", sport)
	cli.NegotiateAPIVersion(ctx)

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
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
		err := errors.New("hct_client container not running\n")
		return err
	}

	xml := fmt.Sprintf("/xml/%s.xml", uuid)
	out := fmt.Sprintf("/output/%s.json", uuid)
	// port := "7070"
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{"/git/voip_patrol/voip_patrol", "--udp",  "--rtp-port", rport,
			"--log-level-file", "1",
			"--log-level-console", "1",
			"--port", sport,
			"--conf", xml,
			"--output", out},
	}

	response, err := cli.ContainerExecCreate(ctx, containerId, execConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		return err
	}
	startConfig := types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	}
	fmt.Printf("ContainerExecCreate [%s]\n", containerId)

	err = cli.ContainerExecStart(ctx, response.ID, startConfig)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
		return err
	}
	fmt.Printf("ContainerExecStart [%s]\n", containerId)

	execInspect, err := cli.ContainerExecInspect(ctx, response.ID)
	fmt.Printf("ContainerExecInspect >> pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
	time.Sleep(time.Duration(expected_duration-1) * time.Second)
	for execInspect.Running {
		time.Sleep(1000 * time.Millisecond)
		execInspect, err = cli.ContainerExecInspect(ctx, response.ID)
		fmt.Printf("ContainerExecInspect >> pid[%d]running[%t]\n", execInspect.Pid, execInspect.Running)
	}
	report, err := resGetReport(uuid)
	rmqPublish("report", report)
	return nil
}


func createXmlFile(uuid string, xml string) (error) {
	// Create file
	dst, err := os.Create("/xml/" + uuid + ".xml")
	if err != nil {
		return err
	}
	defer dst.Close()
	if err := os.WriteFile("/xml/"+uuid+".xml", []byte(xml), 0666); err != nil {
		return err
	}
	return nil
}

func cmdMakeCalls(cmd *Cmd) (error) {
	port_rtp := 10000
	port_sip := 15060
	idx := 0
	calls := cmd.Call.Count
	for calls > 0 {
		n := fmt.Sprintf("%s-%d", cmd.Uuid, idx)
		if calls < 50 {
			params := CallParams{cmd.Call.Ruri,calls-1,cmd.Call.Username,cmd.Call.Password,cmd.Call.Duration,port_rtp,port_sip}
			go cmdCreateCall(n, params)
			calls = 0
		} else {
			params := CallParams{cmd.Call.Ruri,49,cmd.Call.Username,cmd.Call.Password,cmd.Call.Duration,port_rtp,port_sip}
			go cmdCreateCall(n, params)
			calls -= 50
		}
		port_rtp += 200;
		port_sip += 1;
		idx += 1;
	}
	return nil
}

func cmdCreate(s string) (*Cmd, error) {
	cmd := new(Cmd)
	b := []byte(s)

	err := json.Unmarshal(b, cmd)
	if err != nil {
		fmt.Printf("invalid command [%s][%s]\n", cmd, err)
		return nil, err
	}
	if cmd.Call.Count > 1000 {
		fmt.Printf("too many calls requested [%s][%s]\n", cmd.Call.Count, 1000)
		err := errors.New("too many calls requested.")
		return nil, err;
	}
	if cmd.Call.Ruri != "" {
		fmt.Printf("call[%s]\n", cmd.Call.Ruri)
	} else {
		err := errors.New("empy request URI\n")
		return nil, err;
	}
	cmd.Uuid = uuid.NewString()
	return cmd, nil
}

func cmdExec(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s := r.FormValue("cmd")
	cmd, err := cmdCreate(s)
	if err != nil {
		http.Error(w, "cmdCreate error:", http.StatusInternalServerError)
	}
	w.WriteHeader(200)
	w.Write([]byte("<html><a href=\"http://"+os.Getenv("LOCAL_IP")+":8080/res?id="+cmd.Uuid+"\">check report for "+cmd.Uuid+"</a></html>"))
	cmdMakeCalls(cmd)
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

func statsInit(s *Stat, latency int32) {
	s.Stdev = float32(0.0);
	s.m2 = float64(0.0);
	s.Max = latency;
	s.Min = latency;
	s.Average = float32(latency);
	s.Count = 1;
}

func statsUpdate(s *Stat, latency int32) {
	if s.Count == 0 {
		statsInit(s, latency)
		return
	}
	s.Count++
        if s.Min > latency {
                s.Min = latency;
	}
        if s.Max < latency {
                s.Max = latency;
	}

	delta := latency - int32(s.Average);
	var c int32
	if s.Count > 0 {
		c = s.Count
	} else {
		c = 1
	}
	s.Average += float32(delta / c)
	delta2 := latency - int32(s.Average);
	s.m2 += float64(delta*delta2);
	if s.Count-1 > 0 {
		c = s.Count-1
	} else {
		c = 1
	}
	s.Stdev = float32(math.Round((math.Sqrt(s.m2 / float64(c)))*100)/100)
}

func resProcessResultFile(fn string, report *Report) (error) {
	file, err := os.Open("/output/"+fn)
	if err != nil {
		fmt.Printf("error opening result file [%s]\n", err)
		return err;
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
			return err
		}
		if testReport.Action == "call" {
			report.Calls += 1
			if testReport.SipLatency.Invite100Ms > 0 {
				statsUpdate(&report.Sip.Invite100, testReport.SipLatency.Invite100Ms)
			}
			if testReport.SipLatency.Invite18xMs > 0 {
				statsUpdate(&report.Sip.Invite18x, testReport.SipLatency.Invite18xMs)
			}
			if testReport.SipLatency.Invite200Ms > 0 {
				statsUpdate(&report.Sip.Invite200, testReport.SipLatency.Invite200Ms)
			}

			if len(testReport.RtpStats) > 0 {
				report.Rtp.Tx.Pkt += testReport.RtpStats[0].Tx.Pkt
				report.Rtp.Tx.Kbytes += testReport.RtpStats[0].Tx.Kbytes
				report.Rtp.Tx.Lost += testReport.RtpStats[0].Tx.Loss
				if report.Rtp.Tx.JitterMax < testReport.RtpStats[0].Tx.JitterMax {
					report.Rtp.Tx.JitterMax = testReport.RtpStats[0].Tx.JitterMax
				}
				report.Rtp.Rx.Pkt += testReport.RtpStats[0].Rx.Pkt
				report.Rtp.Rx.Kbytes += testReport.RtpStats[0].Rx.Kbytes
				report.Rtp.Rx.Lost += testReport.RtpStats[0].Rx.Loss
				if report.Rtp.Rx.JitterMax < testReport.RtpStats[0].Rx.JitterMax {
					report.Rtp.Rx.JitterMax = testReport.RtpStats[0].Rx.JitterMax
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
		return err
	}
	return nil
}

func resGetReport(uuid string) (string, error) {
	var report Report
	entries, err := os.ReadDir("/output")
	if err != nil {
		fmt.Printf("error opening result directory [%s]\n", err)
		return "", err
	}
	for _, e := range entries {
		s := e.Name()
		if  s[len(s)-5:] == ".json" && strings.Contains(s, uuid) {
			fmt.Println(s)
			err := resProcessResultFile(s, &report)
			if err != nil {
				return "", err
			}
		}
	}
	reportJson, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	fmt.Println(string(reportJson))
	return string(reportJson), nil
}

func resHandler(w http.ResponseWriter, r *http.Request) {
	ua := r.Header.Get("User-Agent")
	m := "res"
	fmt.Printf("[%s] %s...\n", ua, m)

	uuid := r.URL.Query().Get("id")

	if uuid == "" {
		fmt.Printf("missing id parameter\n")
		http.Error(w, "missing id parameter", http.StatusInternalServerError)
		return
	}
	fmt.Println("id =>", uuid)

	report, err := resGetReport(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, report)
}

func cmdCreateCall(uuid string, p CallParams) (error) {
	xml := fmt.Sprintf(`
<config>
  <actions>
    <action type="call" label="%s"
            transport="udp"
            expected_cause_code="200"
            caller="hct_controller@noreply.com"
	    callee="%s"
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
	`, uuid, p.Ruri, p.Ruri, p.Repeat, p.Username, p.Password, p.Duration+2, p.Duration)
	fmt.Printf("%s\n", xml)
	err := createXmlFile(uuid, xml)
	if err != nil {
		return err
	}
	err = cmdDockerExec(uuid, p.PortRtp, p.PortSip, p.Duration)
	if err != nil {
		return err
	}
	time.Sleep(1000 * time.Millisecond)
	return nil
}

func main() {
	version := "0.0.0"
	go rmqSubscribe();
	body := `
{
    "call": {
       "destination": "x@35.183.70.45:5555",
       "username": "default",
       "password": "default",
       "count": 2,
       "duration": 10
    }
}`
	go rmqPublish("commands", body);

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
