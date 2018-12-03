def create_interfaces_json_from_list(interfaces):
    ints_json = ""
    for interface in interfaces:
        if interface[:4] == 'bvi_':
            ints_json += '{ "name": "' + interface + '", "bridged_virtual_interface": true },'
        else:
            ints_json += '{ "name": "' + interface + '" },'
    ints_json = ints_json[:-1]
    return ints_json


