import sys
import os
from subprocess import call


def cut(filename, out):
    with open(filename, 'r') as f:
        flag = False
        codes = []
        for line in f.readlines():
            if line.strip() == '```go':
                flag = True
            elif line.strip() == '```':
                with open(out, 'w') as g:
                    g.write("".join(codes))
                return
            elif flag:
                codes.append(line)


if __name__ == '__main__':
    filename = sys.argv[1]
    out = 'hoge.go'
    cut(filename, out)
    ret = call(['go', 'build', '-o', 'hoge', out])
    os.remove(out)
    os.remove('hoge')
    sys.exit(ret)
