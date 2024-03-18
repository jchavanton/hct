
### client
Container running voip_patrol to act as SIP client.
    ./build.sh && ./run.sh

### controller
Container running the hct_controller to driver the hct_client and generate reports.
    ./build.sh && ./run.sh

Controller reports examples

[report_summary](report_summary.json)

[report_call](report_call.json)


### server
Container using voip_patrol to act as a SIP server
    ./build.sh && ./run.sh
 
### Freeswitch
Container using freeswitch to act as a SIP server
    ./build.sh && ./run.sh

### Kamailio
Container using kamailio to act as a SIP load balanced
    ./build.sh && ./run.sh
