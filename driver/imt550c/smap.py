import requests
from requests.auth import HTTPDigestAuth
import time
import json
import sys

class IMT550C():
  def __init__(self):
    params = self.configure()
    self.uri = params["uri"]
    self.ip = params["IP"]
    self.user = params["username"]
    self.password = params["password"]
    self.sample_rate = params["sample_rate"]
    self.points = [
                    {"name": "cooling_setpoint", "unit": "F", "data_type": "double",
                      "OID": "4.1.6", "range": (45.0,95.0), "access": 6,
                      "devtosmap": lambda x: x/10, "smaptodev": lambda x: x*10}, #thermSetbackCool

                    {"name": "fan_state", "unit": "Mode", "data_type": "long",
                      "OID": "4.1.4", "range": [0,1], "access": 4,
                      "devtosmap":  lambda x: {0:0, 1:0, 2:1}[x], "smaptodev": lambda x: {x:x}[x]}, # thermFanState

                    {"name": "heating_setpoint", "unit": "F", "data_type": "double",
                      "OID": "4.1.5", "range": (45.0,95.0), "access": 6,
                      "devtosmap": lambda x: x/10, "smaptodev": lambda x: x*10}, #thermSetbackHeat

                    {"name": "mode", "unit": "Mode", "data_type": "long",
                      "OID": "4.1.1", "range": [0,1,2,3], "access": 6,
                      "devtosmap": lambda x: x-1, "smaptodev": lambda x: x+1}, # thermHvacMode

                    {"name": "override", "unit": "Mode", "data_type": "long",
                      "OID": "4.1.9", "range": [0,1], "access": 6,
                      "devtosmap": lambda x: {1:0, 3:1, 2:0}[x], "smaptodev": lambda x: {0:1, 1:3}[x]}, # hold/override

                    {"name": "relative_humidity", "unit": "%RH", "data_type": "double",
                      "OID": "4.1.14", "range": (0,95), "access": 0,
                      "devtosmap": lambda x: x, "smaptodev": lambda x: x}, #thermRelativeHumidity

                    {"name": "state", "unit": "Mode", "data_type": "long",
                      "OID": "4.1.2", "range": [0,1,2], "access": 4,
                      "devtosmap":  lambda x: {1:0, 2:0, 3:1, 4:1, 5:1, 6:2, 7:2, 8:0, 9:0}[x],
                      "smaptodev":  lambda x: {x:x}[x]}, # thermHvacState

                    {"name": "temperature", "unit": "F", "data_type": "double",
                      "OID": "4.1.13", "range": (-30.0,200.0), "access": 4,
                      "devtosmap": lambda x: x/10, "smaptodev": lambda x: x*10}, # thermAverageTemp

                    {"name": "fan_mode", "unit": "Mode", "data_type": "long",
                      "OID": "4.1.3", "range": [1,2,3], "access": 6,
                      "devtosmap": lambda x: x, "smaptodev": lambda x: x} # thermFanMode
                  ]

  def get_state(self):
    data = {}
    for p in self.points:
      url = "http://%s/get?OID%s" % (self.ip, p["OID"])
      r = requests.get(url, auth=HTTPDigestAuth(self.user, self.password))
      val = r.content.split('=')[-1]

      if p["data_type"] == "long":
        data[p["name"]] = p["devtosmap"](long(val))
      else:
        data[p["name"]] = p["devtosmap"](float(val))

    data["time"] = int(time.time()*1e9)
    return data

  def set_state(self, request):
    for p in self.points:
      key = p["name"]
      if key in request:
        payload = {"OID"+p["OID"]: int(p["smaptodev"](request[key])), "submit": "Submit"}
        r = requests.get('http://'+self.ip+"/pdp/", auth=HTTPDigestAuth(self.user, self.password), params=payload)
        if not r.ok:
          print r.content

  def configure(self):
    params = None
    with open("params.json") as f:
        try:
            params = json.loads(f.read())
        except ValueError as e:
            print "Invalid parameter file"
            sys.exit(1)

    return dict(params)

if __name__ == '__main__':
  thermostat = IMT550C()
  while True:
    print thermostat.get_state()
    print
