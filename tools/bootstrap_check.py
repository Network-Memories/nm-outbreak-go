"""Independent Python client fixture against one Go process; not an original PS2 client.
Requires cryptography only for this development check, never for the Go service.
"""
import hashlib,socket,struct
from cryptography.hazmat.decrepit.ciphers.algorithms import Blowfish
from cryptography.hazmat.primitives.ciphers import Cipher,modes
from runtime_check import main
from app_check import check_app
from kddi_check import check_kddi,join_existing_room

def crypt(key,raw,decrypt=False):
 engine=Cipher(Blowfish(key),modes.ECB()).decryptor() if decrypt else Cipher(Blowfish(key),modes.ECB()).encryptor()
 return engine.update(raw)+engine.finalize()
SNAP_EVENTS={}
def receive_snap(client,port):
 while True:
  raw=client.recv(2048)
  if struct.unpack_from('>H',raw)[0]&0x8000:
   ack=struct.pack('>HBBIII',0x4010,0,0,struct.unpack_from('>I',raw,4)[0],0,struct.unpack_from('>I',raw,8)[0])+bytes.fromhex('ba476611')
   client.sendto(ack,('127.0.0.1',port))
  assert raw[-4:]==bytes.fromhex('ba476611')
  size,packet,cmd,session,number,ack=struct.unpack('>HBBIII',raw[:16]);assert size&0x3ff==len(raw)-4
  if cmd==0:continue
  return dict(flags=size&0xfc00,cmd=cmd,sid=session,seq=number,ack=ack,payload=raw[16:-4])
def exchange(client,port,flags,command,sid,seq,payload=b''):
 body=struct.pack('>HBBIII',flags|(16+len(payload)),0,command,sid,seq,0)+payload+bytes.fromhex('ba476611')
 client.settimeout(3);client.sendto(body,('127.0.0.1',port))
 expected={0x2c:0x40,0x41:0x2d,1:0x29,2:0x28,4:0x28,6:0x28,7:0x28,0x0c:0x28,0x0d:0x28}.get(command,command)
 while True:
  reply=receive_snap(client,port)
  if reply['cmd']==expected:return reply
  assert reply['cmd'] in (6,7,0x0c),reply['cmd']
  SNAP_EVENTS.setdefault(client,[]).append(reply)
def wait_snap_event(client,port,command):
 queue=SNAP_EVENTS.setdefault(client,[])
 for i,r in enumerate(queue):
  if r['cmd']==command:return queue.pop(i)
 while True:
  r=receive_snap(client,port)
  if r['cmd']==command:return r
  assert r['cmd'] in (6,7,0x0c),r['cmd']
  queue.append(r)

def snap_login(boot,peer,udp,name,password,title,index):
 login=bytearray(104);login[:len(name)]=name.encode();struct.pack_into('>I',login,100,title)
 challenge=exchange(boot,udp[0],0xa000,0x2c,0x026407d0,1,login)
 assert challenge['cmd']==0x40 and challenge['flags']==0x3000 and challenge['seq']==1
 clear=crypt(b'SNAP-SWAN',challenge['payload'],True);assert len(clear)==280
 length=struct.unpack_from('>I',clear,4)[0];seed=clear[8:8+length]
 key=hashlib.sha1(password.encode()+seed).digest();secret=crypt(key,clear[148:276],True)
 answer=crypt(b'SNAP-SWAN',struct.pack('>II128s',0,0,secret))
 success=exchange(boot,udp[0],0xa000,0x41,challenge['sid'],2,answer)
 assert success['cmd']==0x2d and success['flags']==0xf000 and success['seq']==0
 destination=crypt(b'SNAP-SWAN',success['payload'],True)
 assert destination[:40].split(b'\0')[0]==name.encode() and struct.unpack_from('>I',destination,44)[0]==udp[index]
 kics=bytearray(0x128);kics[16:32]=name.encode().ljust(16,b'-')
 admitted=exchange(peer,udp[index],0xa000,0x01,challenge['sid'],1,kics)
 assert admitted['cmd']==0x29 and admitted['sid']==challenge['sid'] and admitted['flags']==0xb000 and admitted['seq']==0
 return admitted



def western_guests(udp,code,password,get,verify):
 variants=[('File1-US',0xCAE0,1,7)] if code=='00000002' else [('File2-US',0xCA19,2,1),('File2-PAL',0xCAF3,2,1)]
 results=[];path='/state/file1' if code=='00000002' else '/state/file2'
 for label,title,index,area in variants:
  with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as boot,socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as peer:
   boot.bind(('127.0.0.1',0));peer.bind(('127.0.0.1',0))
   admitted=snap_login(boot,peer,udp,'NativeGuest',password,title,index);sid=admitted['sid']
   profile=bytearray(240);profile[200]=1
   assert exchange(peer,udp[index],0xa000,0x0c,sid,2,profile)['cmd']==0x28
   assert exchange(peer,udp[index],0xb000,0x06,sid,3,struct.pack('>I',area))['cmd']==0x28
   listing=exchange(peer,udp[index],0xb000,0x0b,sid,4,struct.pack('>I',area))
   assert listing['cmd']==0x0b and len(listing['payload'])==48
   room_id=struct.unpack_from('>I',listing['payload'],44)[0]
   joined=exchange(peer,udp[index],0xa000,0x06,sid,5,struct.pack('>II16s',room_id,0,b'roomkey'))
   assert joined['cmd']==0x28 and struct.unpack('>II',joined['payload'])==(6,0)
   roster=exchange(peer,udp[index],0xa000,0x0a,sid,6,struct.pack('>I',room_id))
   assert roster['cmd']==0x0a and struct.unpack_from('>II',roster['payload'],4)==(2,2)
   assert len(roster['payload'])==12+264*2
   verify('joined')
   members=get(path)['rooms'][0]['members'];assert len(members)==2 and len({m['owner_id'] for m in members})==2
   assert exchange(peer,udp[index],0xa000,0x02,sid,7)['cmd']==0x28
   assert len(get(path)['rooms'][0]['members'])==1
   verify('left')
   SNAP_EVENTS.pop(peer,None)
   results.append(label)
 return results


def exercise(udp,tcp,name,password,get):
 outcomes=[]
 app_outcomes=[check_app(tcp[4],name,build) for build in (1,4,6)]
 cases=[(f'File1-US-{scenario}',0xCAE0,1,'outbreak',7,0x00020000|((scenario-1)<<1),scenario)
        for scenario in range(1,6)]
 cases.extend([('File2-US',0xCA19,2,'outbreak2',1,0x020a0002,1),
               ('File2-PAL',0xCAF3,2,'outbreak2',1,0x020a0002,1)])
 for label,title,index,game,area,stat,scenario in cases:
  with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as boot,socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as peer:
   boot.bind(('127.0.0.1',0));peer.bind(('127.0.0.1',0))
   admitted=snap_login(boot,peer,udp,name,password,title,index)
   profile=bytearray(240);profile[200]=4
   update=exchange(peer,udp[index],0xa000,0x0c,admitted['sid'],2,profile);assert update['cmd']==0x28
   assert exchange(peer,udp[index],0xb000,0x06,admitted['sid'],3,struct.pack('>I',area))['cmd']==0x28
   left=exchange(peer,udp[index],0xb000,0x07,admitted['sid'],4)
   assert left['cmd']==0x28 and left['flags']==0xf000 and left['payload']==struct.pack('>II',0x07,0)
   assert exchange(peer,udp[index],0xb000,0x06,admitted['sid'],5,struct.pack('>I',area))['cmd']==0x28
   create=bytearray(44);struct.pack_into('>I',create,16,2);struct.pack_into('>I',create,40,stat)
   created=exchange(peer,udp[index],0xb000,0x04,admitted['sid'],6,create)
   assert created['cmd']==0x28 and struct.unpack_from('>I',created['payload'],4)[0]>0
   rooms=get('/state/file1' if game=='outbreak' else '/state/file2')['rooms']
   assert len(rooms)==1 and len(rooms[0]['members'])==1 and rooms[0]['members'][0]['name']==name
   [public_room]=get(f'/nm/v1/{game}/state')['rooms']
   [public_host]=public_room['participants']
   assert public_room['scenario_id']==scenario and public_room['status']=='En attente', label
   assert public_host['host'] and public_host['character_portrait_key']=='david', label
   joined_wire=[]
   def verify_western(stage):
    event=wait_snap_event(peer,udp[index],6 if stage=='join' else 7)
    assert event['sid']==admitted['sid']
    if stage=='join':
     assert len(event['payload'])==264 and event['payload'][:16].split(b'\0')[0]==b'NativeGuest'
     joined_wire.append(struct.unpack_from('>I',event['payload'],16)[0])
    else:assert struct.unpack('>I',event['payload'])[0]==joined_wire[0]
   mixed=join_existing_room(tcp,'00000002' if game=='outbreak' else '00000010','NativeGuest',password,get,'/state/file1' if game=='outbreak' else '/state/file2',verify_western)
   logged_out=exchange(peer,udp[index],0xa000,0x02,admitted['sid'],7);assert logged_out['cmd']==0x28
   assert not get('/state/file1' if game=='outbreak' else '/state/file2')['rooms']
   outcomes.append({'variant':label,'password_proof':'PASS','game_socket_handoff':'PASS','native_lobby_leave_and_rejoin':'PASS','profile_and_room':'PASS','kddi_guest_admission_and_leave':mixed,'western_host_join_leave_callbacks':'PASS','logout':'PASS'})
 return {'snap':outcomes,'capcom_app':app_outcomes,'kddi':check_kddi(tcp,name,password,get,lambda code,verify:western_guests(udp,code,password,get,verify))}
if __name__=='__main__':main(exercise)
