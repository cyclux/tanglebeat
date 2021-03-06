package main

var indexPage = `
<!DOCTYPE html>
<html>
    <head>
        <title>Tanglebeat dashboard</title>
        <meta charset="UTF-8">
        <meta name="description" content="Page displays state of the Tanglebeat instance">
        <meta name="keywords" content="IOTA, Tangle, Tanglebeat, crypto, token, metrics">
        <meta name="author" content="lunfardo">
	</head>
	<style>
		table {
			border-collapse: collapse;
		}
		th, td, #runningOnly {
		    font-family: "Liberation Mono", monospace;
			font-size: 12px;
			text-align: left;
			padding: 3px;
            vertical-align: top;
		}
		tr:nth-child(even){background-color: #f2f2f2}
	</style>
	<script type="text/javascript" src="/loadjs">
	</script>
	<body onload=main()>
        <h3> IRI msg inputs (Zero MQ)</h3>
		 <input type="checkbox" id="runningOnly" onclick="clickRunningOnly()">Show running only</input>
		 <table id="maintable" border="1">
			<tr> 
				<td>ZMQ host</td>  
		    	<td>avgBehindSNSec</td> 
     			<td>leaderTXPerc</td>
      			<td>leaderSNPerc</td>
      			<td>lastTXMsecAgo</td>
      			<td>lastSNMsecAgo</td>
      			<td>runningAlreadyMin</td>	
            </tr>
		</table>
		 <table border="0">
			<tr>
				<td><b>Compound IRI msg output (Nanomsg)</b></td>
				<td><b>Buffers</b></td>
				<td><b>Go runtime</b></td>
            </tr>
            <tr>
              <td>
				 <table id="outputtable" border="1">
        		   <tr></tr>
         		</table>
              </td>
              <td>
				 <table id="rtt1" border="1">
        		   <tr></tr>
         		 </table>
              </td>
              <td>
				 <table id="rtt2" border="1">
        		   <tr></tr>
         		 </table>
              <td>
              </td>
	        </tr>
         </table>
        <h3>Senders</h3>
		 <table id="sendertable" border="0">
            <tr></tr>
         </table>
	</body>
</html>
`

var loadjs = `
		var runningOnly = false;
        function clickRunningOnly(){
            var checkBox = document.getElementById("runningOnly");
            runningOnly = checkBox.checked;
		}
		function ts2Time(ts, ago) {
    		var d=new Date(); 
    		var nowTs = d.getTime(); 
            var seconds = Math.floor((nowTs-ts)/1000);

			days = Math.floor(seconds / (24 * 3600));
			days_rem = seconds - days * 24 * 3600;
			hours = Math.floor(days_rem/3600) 

			hours_rem = days_rem - hours * 3600
			minutes = Math.floor(hours_rem/60)
			seconds = hours_rem - minutes * 60
			ret = ""
			if (days > 0) ret = days + " days ";
			if (hours > 0) ret = ret + hours + " h ";
            if (minutes > 0) ret = ret + minutes + " min "
			ret = ret + seconds +" sec";
            if (ago){
                ret = ret + " ago";
            }
			return ret
		}
		function refresh(fun, millis){
			fun();
			setInterval(fun, millis);
		}
		function deleteChildren(obj){
    		while( obj.hasChildNodes() ){
        		obj.removeChild(obj.lastChild);
    		}
		}
		function populateRow(row, data, heading){
            if (heading){
                for (key in data){
                    if (key != "lastErr"){
		                el = document.createElement('td');
    	                el.innerHTML = "<b>" + key + "</b>";
	    	  			row.appendChild(el);
                    }
                }
                return true;
            } 
			if (data.running){
                for (key in data){
                    if (key != "lastErr"){
		                el = document.createElement('td');
                        if (key == "runningSince" || key == "lastHeartbeat"){
                            el.innerHTML = ts2Time(data[key], true);
                        } else {
                           el.innerHTML = data[key];
                        }
      					 row.appendChild(el);
                    }
              	}
                return true;
            }
            if (runningOnly){
   	            return false;
       	    }
			el = document.createElement('td');
	       	el.innerHTML = data["uri"];
   			row.appendChild(el);
	
   	    	el = document.createElement('td');
    	    el.innerHTML = data["running"];
   			row.appendChild(el);

       	    el = document.createElement('td');
   	        el.setAttribute("colspan", Object.keys(data).length - 2)
        	el.innerHTML = data["lastErr"];
   			row.appendChild(el);
            return true;
        }
		function populateRoutineStats(resp){
   			tb = document.getElementById("maintable").tBodies[0];
            deleteChildren(tb);
            first = true
			for (idx in resp){
                if (first){
 			        row = document.createElement('tr');
	                populateRow(row, resp[idx], true)
		            tb.appendChild(row);
                    first = false
                }
    		    row = document.createElement('tr');
	            if (populateRow(row, resp[idx], false)){
		            tb.appendChild(row);
                }
            }
		}
		function populate(tbname, datalist){
   			tb = document.getElementById(tbname).tBodies[0];
            deleteChildren(tb);
            for (key in datalist[0]){
                row = document.createElement('tr');

                cell = document.createElement('td');
                cell.innerHTML = key;
                row.appendChild(cell);
				for (idx in datalist){
	                cell = document.createElement('td');
    	            cell.innerHTML = datalist[idx][key];
        	        row.appendChild(cell);
                }
           	    tb.appendChild(row);
			}
		}

	    function refreshStats(){
    		var xhttp = new XMLHttpRequest();
			xhttp.onreadystatechange = function() {
        		var resp;
        		if (this.readyState == 4){
            		if (this.status == 200) {
		               resp = JSON.parse(this.response);
						populateRoutineStats(resp.zmqInputStats);
  			            populate("outputtable", [resp.zmqOutputStats, resp.zmqOutputStats10min]);
  			            populate("rtt1", [resp.zmqRuntimeStats]);
  			            populate("rtt2", [resp.goRuntimeStats]);
                    }
                }
      	    };
      	    req = "/api1/internal_stats/displayall";
            xhttp.open("GET", req, true);
            xhttp.send();
        }

		var colNames = ["Seq", "Addr idx", "Balance", "Bundle", "Duration", "State", "(Re)attach/<br>Promote", "Last heartbeat"];

		function bundleLink(bundle){
           d = 'https://thetangle.org/bundle/' + bundle;
           s = bundle.substring(0, 12) + "..";
           return '<a href="' + d + '" target="_blank">' + s + '</a>';
		}
		function populateSenderStates(resp){
   			tb = document.getElementById("sendertable").tBodies[0];
            deleteChildren(tb);
			firstRow = true;
			for (r in resp){
                if (firstRow){
	                row = document.createElement('tr');
    				for (i in colNames){
		                cell = document.createElement('td');
    	                cell.innerHTML = '<b>'+colNames[i]+'</b>';
						row.appendChild(cell);
                    }                
                    firstRow = false;
					tb.appendChild(row);
                }
                row = document.createElement('tr');

                cell = document.createElement('td');
                cell.innerHTML = resp[r].seqName;
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = resp[r].index;
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = resp[r].balance;
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = bundleLink(resp[r].bundle);
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = ts2Time(resp[r].startedTs, false);
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = resp[r].state;
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = resp[r].numAttach + "/" + resp[r].numPromo;
    			row.appendChild(cell);

                cell = document.createElement('td');
                cell.innerHTML = ts2Time(resp[r].lastHeartbeat, true)
    			row.appendChild(cell);

				tb.appendChild(row);
           }
		}

	    function refreshSenderStates(){
    		var xhttp = new XMLHttpRequest();
			xhttp.onreadystatechange = function() {
        		var resp;
        		if (this.readyState == 4){
            		if (this.status == 200) {
		               resp = JSON.parse(this.response);
						populateSenderStates(resp);
                    }
                }
      	    };
      	    req = "/api1/senders";
            xhttp.open("GET", req, true);
            xhttp.send();
        }

		function main(){
			refresh(refreshStats, 3000);
			refresh(refreshSenderStates, 3000);
		}
`
var loadJsSenderState = `
	

`
