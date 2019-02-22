import json

# input - json output from vxlan_tunnel_dump, src ip, dst ip, vni
# output - true if tunnel exists, false if not, interface index
def Check_VXLan_Tunnel_Presence(out, src, dst, vni):
    out =  out[out.find('['):out.rfind(']')+1]
    data = json.loads(out)
    present = False
    if_index = -1
    for iface in data:
        if iface["src_address"] == src and iface["dst_address"] == dst and iface["vni"] == int(vni):
            present = True
            if_index  = iface["sw_if_index"]
    return present, if_index

# input - json output from sw_interface_dump, index
# output - interface name
def Get_Interface_Name(out, index):
    out =  out[out.find('['):out.rfind(']')+1]
    data = json.loads(out)
    name = "x"
    for iface in data:
        if iface["sw_if_index"] == int(index):
            name = iface["interface_name"]
    return name

# input - json output from sw_interface_dump, interface name
# output - index
def Get_Interface_Index(out, name):
    out =  out[out.find('['):out.rfind(']')+1]
    data = json.loads(out)
    index = -1
    for iface in data:
        if iface["interface_name"] == name:
            index = iface["sw_if_index"]
    return index

# input - json output from sw_interface_dump, index
# output - whole interface state
def Get_Interface_State(out, index):
    out =  out[out.find('['):out.rfind(']')+1]
    data = json.loads(out)
    state = -1
    for iface in data:
        if iface["sw_if_index"] == int(index):
            state = iface
    return state

# input - mac in dec from sw_interface_dump
# output - regular mac in hex
def Convert_Dec_MAC_To_Hex(mac):
    hexmac=[]
    for num in mac[:6]:
        hexmac.append("%02x" % num)
    hexmac = ":".join(hexmac)
    return hexmac

# input - output from show memif intf command
# output - state info list
def Parse_Memif_Info(info):
    state = []
    socket_id = ''
    sockets_line = []
    for line in info.splitlines():
        if line:
            try:
                _ = int(line.strip().split()[0])
                sockets_line.append(line)
            except ValueError:
                pass
            if (line.strip().split()[0] == "flags"):
                if "admin-up" in line:
                    state.append("enabled=1")
                if "slave" in line:
                    state.append("role=slave")
                if "connected" in line:
                    state.append("connected=1")
            if (line.strip().split()[0] == "socket-id"):
                try:
                    socket_id = int(line.strip().split()[1])
                    state.append("id="+line.strip().split()[3])
                    for sock_line in sockets_line:
                      try:
                           num = int(sock_line.strip().split()[0])
                           if (num == socket_id):
                               state.append("socket=" + sock_line.strip().split()[-1])
                      except ValueError:
                           pass
                except ValueError:
                    pass
    if "enabled=1" not in state:
        state.append("enabled=0")
    if "role=slave" not in state:
        state.append("role=master")
    if "connected=1" not in state:
        state.append("connected=0")
    return state

# input - output from show br br_id detail command
# output - state info list
def Parse_BD_Details(details):
    state = []
    details = "\n".join([s for s in details.splitlines(True) if s.strip("\r\n")])
    line = details.splitlines()[1]
    if (line.strip().split()[6]) in ("on", "flood"):
        state.append("unicast=1")
    else:
        state.append("unicast=0")
    if (line.strip().split()[8]) == "on":
        state.append("arp_term=1")
    else:
        state.append("arp_term=0")
    return state

# input - etcd dump
# output - etcd dump converted to json + key, node, name, type atributes
def Convert_ETCD_Dump_To_JSON(dump):
    etcd_json = '['
    key = ''
    data = ''
    firstline = True
    for line in dump.splitlines():
        if line.strip() != '':
            if line[0] == '/':
                if not firstline:
                    etcd_json += '{"key":"'+key+'","node":"'+node+'","name":"'+name+'","type":"'+type+'","data":'+data+'},'
                key = line
                node = key.split('/')[2]
                name = key.split('/')[-1]
                type = key.split('/')[4]
                data = ''
                firstline = False
            else:
                if line == "null":
                    line = '{"error":"null"}'
                data += line 
    if not firstline:
        etcd_json += '{"key":"'+key+'","node":"'+node+'","name":"'+name+'","type":"'+type+'","data":'+data+'}'
    etcd_json += ']'
    return etcd_json

# input - node name, bd name, etcd dump converted to json, bridge domain dump
# output - list of interfaces (etcd names) in bd
def Parse_BD_Interfaces(node, bd, etcd_json, bd_dump):
    interfaces = []
    bd_dump = json.loads(bd_dump)
    etcd_json = json.loads(etcd_json)
    for int in bd_dump[0]["sw_if"]:
        bd_sw_if_index =  int["sw_if_index"]
        etcd_name = "none"
        for key_data in etcd_json:
            if key_data["node"] == node and key_data["type"] == "status" and "/interface/" in key_data["key"]:
                if "if_index" in key_data["data"]:
                    if key_data["data"]["if_index"] == bd_sw_if_index:
                        etcd_name = key_data["data"]["name"]
        interfaces.append("interface="+etcd_name)
    if bd_dump[0]["bvi_sw_if_index"] != 4294967295:
        bvi_sw_if_index = bd_dump[0]["bvi_sw_if_index"]
        etcd_name = "none"
        for key_data in etcd_json:
            if key_data["node"] == node and key_data["type"] == "status" and "/interface/" in key_data["key"]:
                if "if_index" in key_data["data"]:
                    if key_data["data"]["if_index"] == bvi_sw_if_index:
                        etcd_name = key_data["data"]["name"]
        interfaces.append("bvi_int="+etcd_name)
    else:
        interfaces.append("bvi_int=none")
    return interfaces

# input - bridge domain dump, interfaces indexes
# output - true if bd with int indexes exists, false id bd not exists
def Check_BD_Presence(bd_dump, indexes):
    bd_dump = json.loads(bd_dump)
    present = False
    for bd in bd_dump:
        bd_present = True
        for index in indexes:
            int_present = False
            for bd_int in bd["sw_if"]:
                if bd_int["sw_if_index"] == index:
                    int_present = True
            if int_present == False:
                bd_present = False
        if bd_present == True:
            present = True
    return present

