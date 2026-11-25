"""Disposable loopback runtime checks; no production service or original client is used."""
import json,os,pathlib,signal,socket,struct,subprocess,sys,tempfile,threading,time,urllib.request
repo=pathlib.Path(__file__).resolve().parents[1]
def ports(kind,count):
 held=[]
 for _ in range(count):
  s=socket.socket(socket.AF_INET,kind);s.bind(('127.0.0.1',0));held.append(s)
 numbers=[s.getsockname()[1] for s in held]
 for s in held:s.close()
 return numbers

def main(native_client=None, *, check_active_stun_outage=False, native_controller=None):
 with tempfile.TemporaryDirectory(prefix='nm-outbreak-runtime-') as directory:
  root=pathlib.Path(directory);binary=root/'nm-outbreak';store=root/'accounts.json'
  subprocess.run(['go','build','-o',str(binary),'./cmd/nm-outbreak'],cwd=repo,check=True)
  env=os.environ|{'NM_IDENTITY_MODE':'standalone','NM_ACCOUNT_FILE':str(store),'NM_OUTBREAK_BIND_HOST':'127.0.0.1'}
  created=subprocess.run([str(binary),'account-create','LocalFixture'],input='fixturepwd\n',text=True,capture_output=True,env=env,check=True)
  account=json.loads(created.stdout);assert account['handle'] and account['native_alias']
  guest=subprocess.run([str(binary),'account-create','NativeGuest'],input='fixturepwd\n',text=True,capture_output=True,env=env,check=True)
  guest_account=json.loads(guest.stdout);assert guest_account['user_id']!=account['user_id'] and guest_account['handle']!=account['handle']
  before=store.read_bytes()
  udp=ports(socket.SOCK_DGRAM,3);tcp=ports(socket.SOCK_STREAM,7)
  for name,value in zip(['OPENSNAP_BOOTSTRAP_PORT','OPENSNAP_FILE1_PORT','OPENSNAP_FILE2_PORT'],udp):env[name]=str(value)
  env.pop('NM_STUN_CONTROLE', None)
  env.pop('NM_STUN_CONTROLE_SECOURS', None)
  if native_controller:
   control = root / 'control.sock'
   env['NM_STUN_CONTROLE'] = str(control)
   backup_control = root / 'backup.sock'
   env['NM_STUN_CONTROLE_SECOURS'] = str(backup_control)
   def start_controller(control):
    control.unlink(missing_ok=True)
    child = subprocess.Popen([
     'python3', '-c',
     'import sys, threading; from sondes_natives import demarrer; '
     'demarrer(sys.argv[1], ["127.0.0.1"], list(map(int, sys.argv[2:]))); threading.Event().wait()',
     str(control), *map(str, udp),
    ], cwd=native_controller)
    for _ in range(100):
     if control.is_socket():
      return child
     if child.poll() is not None:
      break
     time.sleep(.02)
    child.terminate()
    child.wait(timeout=5)
    raise RuntimeError('native control socket did not start')
   controller = start_controller(control)
   backup_controller = start_controller(backup_control)
  for name,value in zip(['KDDI_FILE1_LOBBY_PORT','KDDI_FILE1_GAME_PORT','KDDI_FILE2_LOBBY_PORT','KDDI_FILE2_GAME_PORT','OPENSNAP_CAPCOM_APP_PORT'],tcp):env[name]=str(value)
  env['NM_OUTBREAK_WEB_ADDR']=f'127.0.0.1:{tcp[6]}'
  env['NM_OUTBREAK_STATE_ADDR']=f'127.0.0.1:{tcp[5]}';env['NM_STUN_INTERVAL_SEC']='1';env['NM_STUN_TIMEOUT_SEC']='1'
  stun=socket.socket(socket.AF_INET,socket.SOCK_DGRAM);stun.bind(('127.0.0.1',0));stun.settimeout(.2)
  env['NM_STUN_ADDR']='127.0.0.1:'+str(stun.getsockname()[1])
  enabled=threading.Event();enabled.set();stop=threading.Event()
  def responder():
   while not stop.is_set():
    try:raw,remote=stun.recvfrom(512)
    except socket.timeout:continue
    except OSError:return
    if not enabled.is_set() or len(raw)<20:continue
    mapped=bytes(a^b for a,b in zip(bytes([203,0,113,7]),raw[4:8]))
    attr=struct.pack('>HHBBH',0x20,8,0,1,32000^(0x2112))+mapped
    response=struct.pack('>HHI',0x101,len(attr),0x2112a442)+raw[8:20]+attr
    stun.sendto(response,remote)
  worker=threading.Thread(target=responder,daemon=True);worker.start()
  logs=open(root/'runtime.log','w');process=subprocess.Popen([str(binary)],env=env,stdout=logs,stderr=subprocess.STDOUT)
  def get(path):
   with urllib.request.urlopen(f'http://127.0.0.1:{tcp[5]}'+path,timeout=1) as response:return json.load(response)
  def until(predicate,timeout=8):
   deadline=time.monotonic()+timeout
   while time.monotonic()<deadline:
    if process.poll() is not None:raise RuntimeError('service exited unexpectedly; '+(root/'runtime.log').read_text()[-700:])
    try:
     value=predicate()
     if value:return value
    except (OSError,ValueError):pass
    time.sleep(.05)
   raise AssertionError('runtime condition timed out')
  try:
   def healthy_state():
    h=get('/health');return h if h['stun']['healthy'] else None
   healthy=until(healthy_state)
   assert healthy['status']=='degraded' and not healthy['ready'] and not healthy['stun']['relay_enabled']
   if native_controller:
    assert healthy['native_udp'].keys() == {'bootstrap', 'file1', 'file2'}
    assert all(route['connected'] and not route['using_backup'] for route in healthy['native_udp'].values())
   idle_queries=[]
   for port in tcp[:4]:
    with socket.create_connection(('127.0.0.1',port),timeout=1) as client:
     client.settimeout(.15)
     try:first=client.recv(12)
     except socket.timeout:first=b''
     idle_queries.append(bool(first))
   app_listening=True
   try:
    with socket.create_connection(('127.0.0.1',tcp[4]),timeout=.2):pass
   except OSError:app_listening=False
   # A syntactically valid command probe is reconstructed, not an original-client login.
   bootstrap=struct.pack('>HBBIII',0x2010,0,0x2c,1234,1,0)+bytes.fromhex('ba476611')
   udp_answers=[]
   for port in udp:
    with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as client:
     client.settimeout(.15);client.sendto(bootstrap,('127.0.0.1',port))
     try:reply=client.recv(2048)
     except socket.timeout:reply=b''
     udp_answers.append(bool(reply))
   def pause_stun():
    enabled.clear()
    until(lambda: not get('/health')['stun']['healthy'])
   if check_active_stun_outage:
    def interrupt_transports():
     pause_stun()
     if native_controller:
      controller.terminate()
      controller.wait(timeout=5)
      until(lambda: all(route['connected'] and route['using_backup'] for route in get('/health')['native_udp'].values()))
      print('Contrôleur UDP NM : reprise par le secours', file=sys.stderr)
    native_result=native_client(udp,tcp,'LocalFixture','fixturepwd',get,interrupt_transports)
    assert not get('/health')['stun']['healthy']
    enabled.set()
    until(healthy_state)
   else:
    native_result=native_client(udp,tcp,'LocalFixture','fixturepwd',get) if native_client else None
   assert get('/state/file1')['rooms']==[] and get('/state/file2')['rooms']==[]
   if native_controller:
    routes = get('/health')['native_udp']
    assert all(route['received'] > 0 for route in routes.values())
    if native_client:
     assert routes['file2']['sent'] > 0
    print('Contrôleur UDP NM : routes vérifiées', file=sys.stderr)
   pause_stun()
   # Existing engine traffic is covered by the separate core test; check listeners/API survive here.
   with socket.create_connection(('127.0.0.1',tcp[2]),timeout=1):pass
   enabled.set();until(lambda:get('/health')['stun']['healthy'])
   assert before==store.read_bytes(),'runtime probes changed the standalone identities'
   process.send_signal(signal.SIGTERM);process.wait(timeout=5);assert process.returncode==0
   report={'runtime_lifecycle':'PASS','standalone_account_preserved':True,'file1_file2_state_isolated':True,'stun_udp_probe_outage_and_recovery':'PASS with simulated STUN server','health_truthfully_not_ready':True,'clean_sigterm':True,'native_kddi_initial_login_queries_sent':idle_queries,'capcom_app_listening':app_listening,'malformed_bootstrap_probe_received_reply':udp_answers,'independent_native_bootstrap_client':native_result,'native_gameplay_ready':False,'original_clients_tested':False,'production_contacted':False}
   print(json.dumps(report,indent=2))
  finally:
   if process.poll() is None:
    process.terminate()
    try:process.wait(timeout=5)
    except subprocess.TimeoutExpired:process.kill();process.wait()
   stop.set();stun.close();worker.join(1);logs.close()
   if native_controller:
    for child in (controller, backup_controller):
     if child.poll() is None:
      child.terminate()
     child.wait(timeout=5)
if __name__=='__main__':main()
