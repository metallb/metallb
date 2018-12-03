# input - output from sh int addr
# output - list of words containing ip/prefix
def Find_IPV4_In_Text(text):
    ipv4 = []
    for word in text.split():
        if (word.count('.') == 3) and (word.count('/') == 1):
            ipv4.append(word)
    return ipv4


def Find_IPV6_In_Text(text):
    """Find and return all IPv6 addresses in the given string.

    :param text: string to search.
    :type text: str

    :return: IPv6 addresses found in string.
    :rtype: list of str
    """

    ipv6 = []
    for word in text.split():
        if (word.count(':') >= 2) and (word.count('/') == 1):
            ipv6.append(word)
    return ipv6


# input - output from sh hardware interface_name
# output - list of words containing mac
def Find_MAC_In_Text(text):
    mac = ''
    for word in text.split():
        if (word.count(':') == 5):
            mac = word
            break
    return mac


# input - output from sh ip arp command
# output - state info list
def Parse_ARP(info, intf, ip, mac):
    for line in info.splitlines():
        if intf in line and ip in line and mac in line:
            print "ARP Found:"+line
            return True
    print "ARP Found"
    return False


# input - output from sh ip arp command
# output - state info list
def parse_stn_rule(info):
    state = {}
    for line in info.splitlines():
        try:
            if "address" in line.strip().split()[0]:
                state['ip_address'] = line.strip().split()[1]
            elif "iface" in line.strip().split()[0]:
                state['iface'] = line.strip().split()[1]
            elif "next_node" in line.strip().split()[0]:
                state['next_node'] = line.strip().split()[1]
        except IndexError:
            pass

    return state['ip_address'], state['iface'], state['next_node']
