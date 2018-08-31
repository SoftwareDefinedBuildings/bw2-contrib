import time
from xbos.devices.thermostat import Thermostat
from xbos import get_client

def heating(tstat):
    # get setpoint
    old_hsp = tstat.heating_setpoint

    # set new heating setpoint
    print "Setting new heating setpoint to:", old_hsp+5
    tstat.set_heating_setpoint(old_hsp+5)

    # wait for update
    time.sleep(12)

    # check if it changed
    if tstat.heating_setpoint == old_hsp:
        print "BAD HEATING SETPOINT", tstat.heating_setpoint
        return

    # reset to old value
    print "Setting heating setpoint back to:", old_hsp
    tstat.set_heating_setpoint(old_hsp)

    print "===Heating test passes.==="

def cooling(tstat):
    # get setpoint
    old_csp = tstat.cooling_setpoint

    # set new cooling setpoint
    print "Setting new cooling setpoint to:", old_csp-5
    tstat.set_cooling_setpoint(old_csp-5)

    # wait for update
    time.sleep(12)

    # check if it changed
    if tstat.cooling_setpoint == old_csp:
        print "BAD COOLING SETPOINT", tstat.cooling_setpoint
        return

    # reset to old value
    print "Setting cooling setpoint back to:", old_csp
    tstat.set_cooling_setpoint(old_csp)

    print "===Cooling test passes.==="

def override(tstat):
    # get setpoint
    old_or = tstat.override

    # set new override
    print "Setting new override to:", 1-old_or
    tstat.set_override(1-old_or)

    # wait for update
    time.sleep(12)

    # check if it changed
    if tstat.override == old_or:
        print "BAD OVERRIDE", tstat.override
        return

    # reset to old value
    print "Setting override back to:", old_or
    tstat.set_override(old_or)

    print "===Override test passes.==="

def mode(tstat):
    old_mode = tstat.mode

    # set new mode
    print "Setting new mode to:", (old_mode+1)%4
    tstat.set_mode((old_mode+1)%4)

    # wait for update
    time.sleep(12)

    # check if it changed
    if tstat.mode == old_mode:
        print "BAD MODE", tstat.mode
        return

    # reset to old value
    print "Setting mode back to:", old_mode
    tstat.set_mode(old_mode)

    print "===Mode test passes.==="

def fan(tstat):
    # get setpoint
    old_fanmode = tstat.fan_mode
    new_fanmode = 2
    if old_fanmode == 2:
        new_fanmode = 3

    # set new heating setpoint
    print "Setting new fan_mode to:", new_fanmode
    tstat.set_fan_mode(new_fanmode)

    # wait for update
    time.sleep(15)

    # check if it changed
    if tstat.fan_mode == old_fanmode:
        print "BAD FAN MODE", tstat.fan_mode

    # reset to old value
    print "Setting fan mode back to:", old_fanmode
    tstat.set_fan_mode(old_fanmode)

    print "===Fan test passes.==="

if __name__ == '__main__':
    URIs = ["scratch.ns/demo/s.imt550c/410soda/i.xbos.thermostat"]
    c = get_client()

    for uri in URIs:
        tstat = Thermostat(c, uri)

        heating(tstat)
        cooling(tstat)
        override(tstat)
        mode(tstat)
        fan(tstat)
