def extract_logger_level(logger_name, list_jsons):
    print("Extracting logger level for {}".format(logger_name))
    try:
        list_jsons = eval(list_jsons)
        if not isinstance(list_jsons, list):
            print("Given object is not instance of list.")
            return None
    except SyntaxError:
        print("Error: Cannot eval given string to list.")
        return None
    print("Given list is {}".format(list_jsons))
    for item in list_jsons:
        if item['logger'] == logger_name:
            return item['level']
    return None
