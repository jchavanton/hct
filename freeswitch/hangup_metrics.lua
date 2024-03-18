local socket = require("socket")
local https = require("socket.http")
local json = require("json")
local ip = os.getenv("LOCAL_IPV4")
local response_body = {}
-- jitter buffer stats
local size_max_ms = session:getVariable("rtp_jb_size_max_ms");
local size_est_ms = session:getVariable("rtp_jb_size_est_ms");
local acceleration_ms = session:getVariable("rtp_jb_acceleration_ms");
local fast_acceleration_ms = session:getVariable("rtp_jb_fast_acceleration_ms");
local forced_acceleration_ms = session:getVariable("rtp_jb_forced_acceleration_ms");
local buffering_skip = session:getVariable("rtp_jb_buffering_skip");
local expand_ms = session:getVariable("rtp_jb_expand_ms");
local jitter_max_ms = session:getVariable("rtp_jb_jitter_max_ms");
local jitter_est_ms = session:getVariable("rtp_jb_jitter_est_ms");

local reset_count = session:getVariable("rtp_jb_reset_count");
local reset_too_big = session:getVariable("rtp_jb_reset_too_big");
local reset_too_expanded = session:getVariable("rtp_jb_reset_too_expanded");
local reset_missing_frames = session:getVariable("rtp_jb_reset_missing_frames");
local reset_ts_jump = session:getVariable("rtp_jb_reset_ts_jump");
local reset_error = session:getVariable("rtp_jb_reset_error");

local call_id     = session:getVariable("sip_call_id");
local out_call_id = session:getVariable("last_bridge_to");
local direction = session:getVariable("direction");

local ejb = session:getVariable("ejb");
local packet_stats_report_b = session:getVariable("packet_stats_report");
local packet_stats_report_a
local report = "";

local answered_time = session:getVariable("answered_time");

if direction == "outbound" then
	session2 = freeswitch.Session(out_call_id);
	if session2 == nil then
		return
	end
	out_call_id = call_id
	call_id = session2:getVariable("sip_call_id");
	packet_stats_report_b = session2:getVariable("packet_stats_report");
	packet_stats_report_a = session:getVariable("packet_stats_report");
	size_max_ms = session2:getVariable("rtp_jb_size_max_ms");
	size_est_ms = session2:getVariable("rtp_jb_size_est_ms");
	acceleration_ms = session2:getVariable("rtp_jb_acceleration_ms");
	fast_acceleration_ms = session2:getVariable("rtp_jb_fast_acceleration_ms");
	forced_acceleration_ms = session2:getVariable("rtp_jb_forced_acceleration_ms");
	buffering_skip = session2:getVariable("rtp_jb_buffering_skip");
	expand_ms = session2:getVariable("rtp_jb_expand_ms");
	jitter_max_ms = session2:getVariable("rtp_jb_jitter_max_ms");
	jitter_est_ms = session2:getVariable("rtp_jb_jitter_est_ms");
	reset_count = session2:getVariable("rtp_jb_reset_count");
	reset_too_big = session2:getVariable("rtp_jb_reset_too_big");
	reset_too_expanded = session2:getVariable("rtp_jb_reset_too_expanded");
	reset_missing_frames = session2:getVariable("rtp_jb_reset_missing_frames");
	reset_ts_jump = session2:getVariable("rtp_jb_reset_ts_jump");
	reset_error = session2:getVariable("rtp_jb_reset_error");
	answered_time = session:getVariable("answered_time");
else
	session2 = freeswitch.Session(out_call_id);
	if session2 == nil then
		return
	end
	packet_stats_report_a = session2:getVariable("packet_stats_report");
	packet_stats_report_b = session:getVariable("packet_stats_report");
end

function http_post_duration(v)
	session:consoleLog("info", "[http-kamailio]["..direction.."] :".. v .. "\n");
	local r, c, h, s = https.request{
		method = 'POST',
		url = "http://"..ip..":80/freeswitch_cdr",
		headers = {
			["Content-Type"] = "application/json",
			["Content-Length"] = string.len(v)
		},
		source = ltn12.source.string(v),
		sink = ltn12.sink.table(response_body)
	}
end

local ts = socket.gettime()*1000;
local duration = (ts - (answered_time/1000))/1000;
duration = math.floor(duration+0.5);
if direction == "inbound" then
	local cdr = "{\"duration\":"..duration..",\"ejb\":\""..ejb.."\",\"call-id\":\""..call_id.."\"}"
	http_post_duration(cdr)
end

if packet_stats_report_b == nil then
	print("direction:"..direction.." duration:"..duration.." ejb:"..ejb.." call-id:"..call_id.." packet_stats_report_b missing\n");
	print("duration:"..duration.." ejb:"..ejb.." call-id:"..call_id.." packet_stats_report_a:"..packet_stats_report_a.."\n");
	return
end
print("direction:"..direction.." duration:"..duration.." ejb:"..ejb.."\n");

local r = "{\"duration\": "..duration..",\"call_id_in\": \""..out_call_id.."\",  \"call_id_out\": \""..call_id.."\", \"report_a\": "..packet_stats_report_a..", \"report_b\": "..packet_stats_report_b.."}"

function http_post(v)
	session:consoleLog("info", "[http-kamailio]["..direction.."] :".. v .. "\n");
	local r, c, h, s = https.request{
		method = 'POST',
		url = "http://"..ip..":80/freeswitch_packet_metrics",
		headers = {
			["Content-Type"] = "application/json",
			["Content-Length"] = string.len(v)
		},
		source = ltn12.source.string(v),
		sink = ltn12.sink.table(response_body)
	}
end

-- if ejb == "true" then
-- 	local v = json.decode(r)
-- 	local avg = v["report_a"]["out"]["avg"]
-- 	if avg > 2400 then
-- 		https.request {
-- 			method = 'GET',
-- 			url = "http://"..ip..":80/disable_ejb",
-- 			sink = ltn12.sink.table(response_body)
-- 		}
-- 		print("disabling EJB due to abnormally high latency, avg[" .. avg .. "]\n")
-- 	end
-- end

if size_max_ms == nil or size_est_ms == nil or acceleration_ms == nil or expand_ms == nil or jitter_max_ms == nil or jitter_est_ms == nil then
	size_max_ms = 0
	size_est_ms = 0
	acceleration_ms = 0
	expand_ms = 0
	jitter_max_ms = 0
	jitter_est_ms = 0
end
local request_body = '{"in_call_id": "'..call_id..'", "out_call_id": "'..out_call_id..'", "jb":{"size_max_ms":'..size_max_ms..
                      ',"size_est_ms":'..size_est_ms..',"acceleration_ms":'..acceleration_ms..',"expand_ms":'..expand_ms..
                      ',"fast_acceleration_ms":'..fast_acceleration_ms..',"forced_acceleration_ms":'..forced_acceleration_ms..
		      ',"jitter_max_ms":'..jitter_max_ms..',"jitter_est_ms":'..jitter_est_ms..',"reset":'..reset_count
request_body = request_body .. ',"reset_too_big":'..reset_too_big
request_body = request_body .. ',"reset_too_expanded":'..reset_too_expanded
request_body = request_body .. ',"reset_missing_frames":'..reset_missing_frames
request_body = request_body .. ',"reset_ts_jump":'..reset_ts_jump
request_body = request_body .. ',"reset_error":'..reset_error
request_body = request_body .. ',"buffering_skip":'..buffering_skip
local v = request_body .. '}}';

report = '{ "ejb": "'..ejb..'", "rp": '..r..', "jb": '..v..'}';
http_post(report)
