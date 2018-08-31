from bw2python import ponames
from bw2python.bwtypes import PayloadObject
from bw2python.client import Client
import smap
import msgpack
import datetime
import time

bw_client = Client()
bw_client.setEntityFromEnviron()
bw_client.overrideAutoChainTo(True)
thermostat = smap.IMT550C()

def toHandle(bw_message):
  for po in bw_message.payload_objects:
    if po.type_dotted == (2, 1, 1, 0):
      to_process = msgpack.unpackb(po.content)
      print to_process
      thermostat.set_state(to_process)

bw_client.subscribe('{0}/slot/state'.format(thermostat.uri), toHandle)

while True:
  msg = thermostat.get_state()
  po = PayloadObject((2, 1, 1, 0), None, msgpack.packb(msg))
  bw_client.publish('{0}/signal/info'.format(thermostat.uri), payload_objects=(po,), persist=True)
  time.sleep(thermostat.sample_rate)

  #RFC 3339 timestamp UTC
  d = datetime.datetime.utcnow()
  timestamp = {'ts': int(time.time()*1e9), 'val': d.isoformat('T')}
  po2 = PayloadObject((2, 0, 3, 1), None, msgpack.packb(timestamp))
  bw_client.publish('{0}/!meta/lastalive'.format(thermostat.uri), payload_objects=(po2,), persist=True)
