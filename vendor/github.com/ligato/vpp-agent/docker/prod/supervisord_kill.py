#!/usr/bin/env python

import sys
import os
import signal


def write_stdout(msg):
    # only eventlistener protocol messages may be sent to stdout
    sys.stdout.write(msg)
    sys.stdout.flush()


def write_stderr(msg):
    sys.stderr.write(msg)
    sys.stderr.flush()


def main():
    while 1:
        # transition from ACKNOWLEDGED to READY
        write_stdout('READY\n')

        # read header line and print it to stderr
        line = sys.stdin.readline()
        write_stderr('EVENT: ' + line)

        # read event payload and print it to stderr
        headers = dict([x.split(':') for x in line.split()])
        data = sys.stdin.read(int(headers['len']))
        write_stderr('DATA: ' + data + '\n')

        # ignore non vpp events, skipping
        parsed_data = dict([x.split(':') for x in data.split()])
        if parsed_data["processname"] not in ["vpp", "agent"]:
            write_stderr('Ignoring event from ' + parsed_data["processname"] + '\n')
            write_stdout('RESULT 2\nOK')
            continue

        # ignore exits with expected exit codes
        if parsed_data["expected"] == "1":
            write_stderr('Exit state from ' + parsed_data["processname"] + ' was expected\n')
            write_stdout('RESULT 2\nOK')
            continue

        # do not kill supervisor if retained and exit
        if 'RETAIN_SUPERVISOR' in os.environ and os.environ['RETAIN_SUPERVISOR'] != '':
            write_stderr('Supervisord is configured to retain after unexpected exits (unset RETAIN_SUPERVISOR to disable it)\n')
            write_stdout('RESULT 2\nOK')
            continue

        try:
            with open('/run/supervisord.pid', 'r') as pidfile:
                pid = int(pidfile.readline())
            write_stderr('Killing supervisord with pid: ' + str(pid) + '\n')
            os.kill(pid, signal.SIGQUIT)
        except Exception as e:
            write_stderr('Could not kill supervisor: ' + str(e) + '\n')

        # transition from READY to ACKNOWLEDGED
        write_stdout('RESULT 2\nOK')
        return


if __name__ == '__main__':
    main()
