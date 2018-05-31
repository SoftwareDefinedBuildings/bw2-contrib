# tests how long it takes to change a value on the thermostat
import smap
import requests
from requests.auth import HTTPDigestAuth
import time

thermostat = smap.IMT550C()

#get old value
url = "http://%s/get?OID%s" % (thermostat.ip, "4.1.5")
r = requests.get(url, auth=HTTPDigestAuth(thermostat.user, thermostat.password))
old = r.content.split('=')[-1]

#set new value
new = 600
payload = {"OID"+"4.1.5": new, "submit": "Submit"}
r = requests.get('http://'+thermostat.ip+"/pdp/", auth=HTTPDigestAuth(thermostat.user, thermostat.password), params=payload)

start = time.time()
curr = old

while curr != new:
    r = requests.get(url, auth=HTTPDigestAuth(thermostat.user, thermostat.password))
    curr = int(r.content.split('=')[-1])

end = time.time()

#set back old value
payload = {"OID"+"4.1.5": old, "submit": "Submit"}
r = requests.get('http://'+thermostat.ip+"/pdp/", auth=HTTPDigestAuth(thermostat.user, thermostat.password), params=payload)

print "Thermostat delay:", end-start, "seconds"
