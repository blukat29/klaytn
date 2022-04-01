import re

started = False
for line in open('interface.go').readlines():
    if line.startswith('type HeaderEngine interface {'):
        started = True
    if not started:
        continue

    m = re.search(r'([^\(]+)\(([^\(]*)\)(.*)', line.strip())
    if not m:
        continue
    name, args, ret = m.groups()
    print('func (e *MixedEngine) %s(%s)%s {' % (name, args, ret))
    arg_items = args.split(',')
    arg_names = map(lambda it: it.strip().split(' ')[0], arg_items)
    arg_calls = ', '.join(arg_names)
    if ret == '':
        print('\te.defaultGov.%s(%s)' % (name, arg_calls))
    else:
        print('\treturn e.defaultGov.%s(%s)' % (name, arg_calls))
    print('}')

