# EMU code from https://github.com/rainforestautomation/Emu-Serial-API
from emu import *
import sys
import json
import msgpack
from xbos import get_client
from bw2python.bwtypes import PayloadObject
import time

with open("params.json") as f:
    try:
        params = json.loads(f.read())
    except ValueError as e:
        print "Invalid parameter file"
        sys.exit(1)       


emu_instance = emu(params["port"])
emu_instance.start_serial()

# get network info
emu_instance.get_network_info()
while not hasattr(emu_instance, 'NetworkInfo'):
    time.sleep(10)
    macid =  emu_instance.NetworkInfo.DeviceMacId

c = get_client(agent=params["agent"], entity=params["entity"])

PONUM = (2,0,9,1)
baseuri = params["baseuri"]
signaluri = "{0}/s.emu2/{1}/i.meter/signal/meter".format(baseuri, macid)
print ">",signaluri
def send_message(msg):
    """
    msg has keys:
    current_demand
    current_price
    current_tier
    current_summation_delivered
    current_summation_received
    """
    po = PayloadObject(PONUM, None, msgpack.packb(msg))
    c.publish(signaluri, payload_objects=(po,))

msg = {}
while True:
    #print emu_instance.get_instantaneous_demand()
    emu_instance.get_current_summation_delivered()
    emu_instance.get_instantaneous_demand('Y')
    emu_instance.get_current_price('Y')
    time.sleep(10)
    
    msg['current_time'] = time.time()#int(pc.TimeStamp)   + 00:00:00 1 Jan 2000
    # handle PriceCluster
    if hasattr(emu_instance, "PriceCluster"):
        pc = emu_instance.PriceCluster
        print dir(emu_instance.PriceCluster)
        msg['current_price'] = float(int(pc.Price, 16)) / (10**int(pc.TrailingDigits,16))
        msg['current_tier'] = int(pc.Tier, 16)
    
    # handle demand
    if hasattr(emu_instance, "InstantaneousDemand"):
        d = emu_instance.InstantaneousDemand
        msg['current_demand'] = int(d.Demand, 16)
    print dir(emu_instance)
    # handle current summation
    if hasattr(emu_instance, "CurrentSummationDelivered"):
        d = emu_instance.CurrentSummationDelivered
        multiplier = int(d.Multiplier, 16)
        divisor = float(int(d.Divisor, 16))
        msg['current_summation_delivered'] = int(d.SummationDelivered, 16) * multiplier / divisor
        msg['current_summation_received'] = int(d.SummationReceived, 16) * multiplier / divisor
    send_message(msg)
emu_instance.stop_serial()
