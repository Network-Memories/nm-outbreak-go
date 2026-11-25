"""Independent reconstructed KDDI native login; not an original PS2 client."""
import socket,struct,urllib.parse,urllib.request,re
from app_check import readn

def frame(peer,sender,kind,cmd,pid,payload=b''):
 peer.sendall(struct.pack('>BBHHHB3s',sender,kind,cmd,len(payload),pid,0,b'\xff'*3)+payload)
def receive(peer):
 h=readn(peer,12);sender,kind,cmd,length,pid,error,reserved=struct.unpack('>BBHHHB3s',h)
 return sender,kind,cmd,pid,error,readn(peer,length)
def reply(peer,command,pid,events):
 while True:
  result=receive(peer)
  if result[1]==0x10:events.append(result);continue
  assert result[:5]==(0x18,2,command,pid,0),result[:5]
  return result[5]
def wait_event(peer,command,events):
 for i,event in enumerate(events):
  if event[2]==command:return events.pop(i)
 while True:
  event=receive(peer);assert event[0]==0x18 and event[1]==0x10
  if event[2]==command:return event
  events.append(event)

def native_field(plain,pid):
 fix=(21,23,10,17,23,19,6,13);masks=(0x33,0x30,0x3c,0x34,0x2d,0x30,0x3c,0x34);p=pid&255
 data=bytes(v^((fix[i&7]-(i&0xf8)-p+((p-9+i)&masks[i&7])*2)&255) for i,v in enumerate(plain))
 return struct.pack('>HH',len(plain)+2,0)+data

def native_login(peer,ticket,character=4,lobby_suffix=b'',file2_catalog=True):
 sender,kind,cmd,pid,error,payload=receive(peer);assert (sender,kind,cmd,error,payload)==(0x18,1,0x6101,0,b'\x28\x37')
 digits=f'{int(ticket[:4])+pid:05d}{int(ticket[4:])+pid:05d}'.encode()
 frame(peer,0x81,2,0x6101,pid,b'\0\x0a'+digits+lobby_suffix)
 sender,kind,cmd,pid,error,payload=receive(peer);assert cmd==0x6103
 if lobby_suffix==b'\0\2\0\0':
  frame(peer,0x81,1,0x61a0,1,b'\1');assert receive(peer)[2]==0x61a0
  for probe_id in range(2,6):
   frame(peer,0x81,1,0x600e,probe_id,native_field(b'0',probe_id))
   check=receive(peer);assert check[2]==0x600e and check[5]==b'\0\1' + b'0'
  frame(peer,0x81,2,0x6103,6,bytes(33))
 else:frame(peer,0x81,2,cmd,pid)
 sender,kind,cmd,pid,error,payload=receive(peer);assert (kind,cmd)==(16,0x6131)
 assert payload[0]==1 and payload[1:3]==b'\0\6';hn=payload[3:9];size=struct.unpack_from('>H',payload,9)[0];nickname=payload[11:11+size]
 frame(peer,0x81,1,0x6132,7,native_field(hn,7)+native_field(nickname,7));assert receive(peer)[2]==0x6132;assert receive(peer)[2]==0x6104
 if lobby_suffix and not file2_catalog:
  frame(peer,0x81,1,0x6002,17)
  reset=receive(peer);assert reset[2:]==(0x6002,17,0,b'')
 frame(peer,0x81,1,0x614c,18);motd=receive(peer);assert motd[2]==0x614c and motd[4]==0 and motd[5]==b'\0\0\0'
 profile=bytearray(208);profile[200]=character;frame(peer,0x81,1,0x6190,8,native_field(profile,8));assert receive(peer)[2]==0x6190
 if lobby_suffix and file2_catalog:
  frame(peer,0x81,1,0x6881,19);catalog=receive(peer);assert catalog[2]==0x6881 and catalog[4]==0 and catalog[5]==b'\0'
  for scenario in range(11):
   frame(peer,0x81,1,0x6145,20+scenario,struct.pack('>H',scenario));ranking=receive(peer);assert ranking[2]==0x6145 and ranking[4]==0xff and ranking[5]==b''


def check_kddi(tcp,name,password,get=None,western_guest=None,*,file1_scenario=2,file2_scenario=1):
 outcomes=[]
 for code,index in [('00000002',0),('00000010',2)]:
  post=urllib.parse.urlencode({'username':name,'password':password}).encode()
  with urllib.request.urlopen(f'http://127.0.0.1:{tcp[6]}/{code}/login-form',data=post,timeout=3) as response:body=response.read()
  ticket=re.search(rb'sessid=([0-9]{8})',body).group(1).decode()
  with urllib.request.urlopen(f'http://127.0.0.1:{tcp[6]}/{code}/startsession?sessid={ticket}',timeout=3) as response:assert b'CRS-game-start' in response.read()
  with socket.create_connection(('127.0.0.1',tcp[index]),timeout=3) as peer:
   native_login(peer,ticket,lobby_suffix=b'\0\2\0\0',file2_catalog=code=='00000010')
   for command,query,want in [(0x6203,b'',struct.pack('>H',2 if code=='00000002' else 10)),(0x6207,struct.pack('>H',2 if code=='00000002' else 1),None),(0x6301,b'',struct.pack('>H',10 if code=='00000002' else 6)),(0x6305,b'\0\1',b'\0\1'),(0x6401,b'',b'\0\x14'),(0x6402,b'\0\1',b'\0\1\0\0')]:
    frame(peer,0x81,1,command,9,query);response=receive(peer);assert response[2]==command and response[4]==0
    if want is not None:assert response[5]==want
   creation=None
   if get is not None:
    events=[]
    def query(command,payload=b''):
     frame(peer,0x81,1,command,31,payload)
     return reply(peer,command,31,events)
    path='/state/file1' if code=='00000002' else '/state/file2'
    assert query(0x6407,b'\0\1')==b'\0\1'
    assert not get(path)['rooms'],'draft was exposed as a party'
    assert query(0x6404,b'\0\1')==b'\0\1\2'
    media=b'\0\x01' if code=='00000002' else b'\0\x11'
    scenario = file1_scenario if code=='00000002' else file2_scenario
    selection = struct.pack('>H', scenario-1 if code=='00000002' else scenario)
    assert query(0x6509,media+selection)==b'\0\1'+media+selection
    query(0x6609,native_field(b'nativehost',31));query(0x660a,native_field(b'roomkey',31))
    for rule,value in [(0,0),(1,1),(2,1)]:assert query(0x660b,bytes([rule,value]))==b'\1'
    assert query(0x6504,b'\1')==b'\1'
    assert query(0x6504,b'\1')==b'\1','finalization replay'
    assert len(get(path)['rooms'])==1 and len(get(path)['rooms'][0]['members'])==1
    game='outbreak' if code=='00000002' else 'outbreak2'
    [public_room]=get(f'/nm/v1/{game}/state')['rooms']
    [public_host]=public_room['participants']
    # KDDI File #1 encodes the selected scenario from zero, unlike File #2.
    assert public_room['scenario_id']==scenario and public_room['status']=='En attente', code
    assert public_host['host'] and public_host['region']=='ntsc-j' and public_host['character_portrait_key']=='david', code
    def verify(stage):
     if stage=='joined':
      event=wait_event(peer,0x6503,events);assert event[5][:2]==b'\0\6'
      assert query(0x6911)==b'\2' and query(0x6913,b'\2')[:2]==b'\2\1'
     else:
      event=wait_event(peer,0x6502,events);assert event[5][:2]==b'\0\6'
      assert query(0x6911)==b'\1'
    creation={'staged_native_creation':'PASS','native_host_join_leave_broadcasts':'PASS','western_guests':western_guest(code,verify) if western_guest else []}
    query(0x6501);assert not get(path)['rooms']
   if code=='00000010':
    frame(peer,0x81,1,0x6209,60,bytes(6));exit_reply=reply(peer,0x6209,60,[]);assert exit_reply==b''
   frame(peer,0x81,1,0x6006,61);logout=reply(peer,0x6006,61,[]);assert logout==b''
   outcomes.append({'game_code':code,'web_password_proof':'PASS','crs_ticket':'PASS','native_login_version_hn_profile':'PASS','room_query':'PASS','creation_and_reverse_admission':creation,'original_client':False})
 return outcomes


def join_existing_room(tcp,code,name,password,get,state_path,verify_western=None):
 post=urllib.parse.urlencode({'username':name,'password':password}).encode()
 with urllib.request.urlopen(f'http://127.0.0.1:{tcp[6]}/{code}/login-form',data=post,timeout=3) as response:body=response.read()
 ticket=re.search(rb'sessid=([0-9]{8})',body).group(1).decode()
 with socket.create_connection(('127.0.0.1',tcp[0 if code=='00000002' else 2]),timeout=3) as peer:
  native_login(peer,ticket,1,lobby_suffix=b'\0\2\0\0' if code=='00000010' else b'')
  events=[]
  def query(command,payload=b''):
   frame(peer,0x81,1,command,23,payload)
   return reply(peer,command,23,events)
  query(0x6207,struct.pack('>H',2 if code=='00000002' else 1));query(0x6305,b'\0\1')
  preview=query(0x640a,b'\0\1');assert preview[:4]==b'\0\1\3\1'
  # The room belongs to a different proved owner, through the SNAP transport.
  request=b'\0\1'+native_field(b'',23)
  assert query(0x6406,request)==b'\0\1'
  assert query(0x6406,request)==b'\0\1','idempotent native join'
  rooms=get(state_path)['rooms'];assert len(rooms)==1 and len(rooms[0]['members'])==2
  assert len({m['owner_id'] for m in rooms[0]['members']})==2
  assert query(0x6911)==b'\2' and query(0x6912)==b'\2'
  host=query(0x6913,b'\1');assert host[:2]==b'\1\1' and host[-3:]==b'\0\0\6'
  if verify_western:verify_western('join')
  query(0x6501)
  if verify_western:verify_western('leave')
  assert len(get(state_path)['rooms'][0]['members'])==1
 return 'PASS (reconstructed clients, two proved owners)'
