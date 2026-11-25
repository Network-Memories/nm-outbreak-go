"""Independent loopback File2 Western-host/KDDI launch; reconstructed clients only."""
import socket,struct,urllib.request,urllib.parse,re
from functools import partial
from runtime_check import main
from bootstrap_check import snap_login,exchange,receive_snap,wait_snap_event
from kddi_check import native_login,native_field,frame,receive,reply,wait_event
from app_check import readn

def exercise(udp,tcp,name,password,get,pause_stun=None,*,lobby_only=False):
 outcomes=[]
 cases = [
  ('File2-US', 0xCA19, 1, 1, 4, 0, 0),
  ('File2-PAL', 0xCAF3, 1, 1, 4, 0, 0),
  ('File2-PAL-Infinity', 0xCAF3, 5, 1, 6, 1, 1),
  ('File2-PAL-Elimination1', 0xCAF3, 8, 6, 4, 0, 0),
  ('File2-PAL-Showdown1', 0xCAF3, 9, 9, 4, 0, 0),
 ]
 if lobby_only:
  cases.extend(
   (f'File2-PAL-Standard-{scenario}', 0xCAF3, 1, scenario, 4, 0, 0)
   for scenario in range(2,6)
  )
  cases.extend([
   ('File2-PAL-Elimination2', 0xCAF3, 8, 7, 4, 0, 0),
   ('File2-PAL-Elimination3', 0xCAF3, 8, 8, 4, 0, 0),
   ('File2-PAL-Showdown2', 0xCAF3, 9, 10, 4, 0, 0),
   ('File2-PAL-Showdown3', 0xCAF3, 9, 11, 4, 0, 0),
  ])
  cases.extend(
   (label.replace('-PAL-', '-US-'), 0xCA19, area, scenario, rows, nightmare, infinity)
   for label, _, area, scenario, rows, nightmare, infinity in tuple(cases)
   if label.startswith('File2-PAL-')
  )
 for label,title,area,scenario,rows,nightmare,infinity in cases:
  with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as boot,socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as host:
   boot.bind(('127.0.0.1',0));host.bind(('127.0.0.1',0));host.settimeout(3)
   login=snap_login(boot,host,udp,name,password,title,2);sid=login['sid']
   prop=bytearray(240);prop[224]=rows;prop[234]=1;prop[235]=2;prop[236]=1
   prop[237]=nightmare;prop[238]=infinity
   assert exchange(host,udp[2],0xa000,0x0c,sid,2,prop)['cmd']==0x28
   assert exchange(host,udp[2],0xb000,6,sid,3,struct.pack('>I',area))['cmd']==0x28
   create=bytearray(44);struct.pack_into('>I',create,16,2);struct.pack_into('>I',create,40,0x020a0800|(scenario<<1))
   assert exchange(host,udp[2],0xb000,4,sid,4,create)['cmd']==0x28
   post=urllib.parse.urlencode({'username':'NativeGuest','password':password}).encode()
   with urllib.request.urlopen(f'http://127.0.0.1:{tcp[6]}/00000010/login-form',data=post,timeout=3) as response:ticket=re.search(rb'sessid=([0-9]{8})',response.read()).group(1).decode()
   with socket.create_connection(('127.0.0.1',tcp[2]),timeout=3) as jp:
    native_login(jp,ticket,1,lobby_suffix=b'\0\2\0\0');events=[]
    def query(cmd,payload=b''):
     frame(jp,0x81,1,cmd,23,payload);return reply(jp,cmd,23,events)
    query(0x6207,struct.pack('>H',area));query(0x6305,b'\0\1');query(0x640a,b'\0\1')
    query(0x6406,b'\0\1'+native_field(b'',23))
    joined=wait_snap_event(host,udp[2],6);guest_sid=struct.unpack_from('>I',joined['payload'],16)[0]
    guest_prop=joined['payload'][24:];assert len(guest_prop)==240
    seq=4
    def send(tag,body=b''):
     nonlocal seq
     seq+=1;payload=struct.pack('>II',1,guest_sid)+struct.pack('<I',tag)+body
     host.sendto(struct.pack('>HBBIII',0xa000|(16+len(payload)),0,0x10,sid,seq,0)+payload+bytes.fromhex('ba476611'),('127.0.0.1',udp[2]))
    send(1,bytes(8))
    # Original native guest receives the actual host greeting first, then
    # refreshes its roster and confirms6504 (golden native capture).
    greeting=wait_event(jp,0x670e,events)[5]
    host_handle=get('/state/file2')['rooms'][0]['members'][0]['handle'].encode()
    assert greeting==b'\0\6'+host_handle+b'\0\x08'+bytes(8)
    assert query(0x640a,b'\0\1')[:4]==b'\0\1\3\2'
    assert query(0x6412,b'\0\1')==bytes([0,1,0,0,0,0])
    query(0x6504,b'\1')
    assert wait_event(jp,0x6506,events)[5]==bytes([0,2,0,0])
    rule_count = 6 if infinity else 3
    assert query(0x6603,b'\0\1')==bytes([rule_count]), label
    assert query(0x6606,b'\0\1\1')==bytes([1,1])
    assert query(0x6606,b'\0\1\2')==bytes([2,2])
    timer=query(0x6409,b'\0\1');assert timer[:2]==b'\0\1' and 275<=struct.unpack_from('>H',timer,2)[0]<=300
    if lobby_only:
     [room] = get('/nm/v1/outbreak2/state')['rooms']
     assert (room['area_id'], room['scenario_id'], room['status']) == (area, scenario, 'En attente')
     assert [p['character_portrait_key'] for p in room['participants']] == ['kevin', 'mark']
     assert [p['host'] for p in room['participants']] == [True, False]
     assert exchange(host,udp[2],0xa000,2,sid,seq+1)['cmd']==0x28
     assert not get('/nm/v1/outbreak2/state')['rooms']
     outcomes.append({'variant':label,'area_id':area,'scenario_id':scenario,'portraits':['kevin','mark'],'closed':True})
     continue
    query(0x6504,b'\1')
    jp.settimeout(.1)
    try:unexpected=jp.recv(1);raise AssertionError(('replayed preparation callback',unexpected))
    except socket.timeout:pass
    finally:jp.settimeout(3)
    send(3)
    assert wait_event(jp,0x6910,events)[5]==b''
    # First actual launch query proves readiness; no synthetic membership ACK.
    frame(jp,0x81,1,0x6911,23)
    ack=receive_snap(host,udp[2]);assert ack['cmd']==0x10 and ack['sid']==guest_sid and ack['flags']==0xa000
    assert ack['payload']==struct.pack('>II',1,0)+struct.pack('<I',4)+bytes(768)
    # The relayed sender differs from the proved recipient; ACK the host route.
    host.sendto(struct.pack('>HBBIII',0x4010,0,0,sid,0,ack['seq'])+bytes.fromhex('ba476611'),('127.0.0.1',udp[2]))
    send(5,bytes([1,2]))
    id16=bytes(0x30+int(c,16) for c in f'{guest_sid:08x}').ljust(16,b'\0')
    for i,native_name in enumerate([name,'NativeGuest']):send(6,bytes([i,0])+id16+native_name.encode().ljust(16,b'\0'))
    for i,profile in enumerate([prop,guest_prop]):
     for step in range(5):
      size=40 if step==4 else 50
      send(7,bytes([i,size])+struct.pack('<H',step*50)+profile[step*50:step*50+size].ljust(50,b'\0'))
    send(8)
    assert reply(jp,0x6911,23,events)==b'\2'
    assert query(0x6912,b'\0')==b'\2'
    rules=query(0x6914);assert len(rules)==34 and rules[3]==2 and rules[4]==1
    assert rules[7:9]==bytes([nightmare,infinity]), (label,rules[7:9])
    number=query(0x6915);assert len(number)==19 and number[:2]==b'\0\x0f' and number[2:17].isdigit() and number[-2:]==b'\0\0'
    assert query(0x6915)==number,'native session changed on retry'
    gs=query(0x6916);assert gs[:6]==b'\0\4\x7f\0\0\1' and struct.unpack_from('>H',gs,8)[0]==tcp[3]
    profile=query(0x6913,b'\1');assert profile[:2]==b'\1\1' and prop[:208] in profile,(profile[:40],len(profile))
    assert query(0x6917,b'\1')==bytes([1,0,0])
    assert query(0x6917,b'\2')==bytes([2,0,0])
    query(0x6002)
    room = get('/state/file2')['rooms'][0]
    assert (room['area'], room['scenario'], room['playing']) == (area, scenario, True), label
   # The original client closes its lobby to open its own game socket. Its seat remains.
   assert len(get('/state/file2')['rooms'][0]['members'])==2
   seq+=1
   assert exchange(host,udp[2],0xa000,0x0d,sid,seq,struct.pack('>I',1))['cmd']==0x28
   # Engine data reaches the common mailbox before the native game socket opens.
   early=b'early-body\x00\xff'
   seq+=1
   payload=struct.pack('<H',len(early))+early
   host.sendto(struct.pack('>HBBIII',0xa000|(16+len(payload)),0,15,sid,seq,0)+payload+bytes.fromhex('ba476611'),('127.0.0.1',udp[2]))
   while True:
    packet=host.recv(2048)
    if packet[3]==0 and struct.unpack_from('>I',packet,12)[0]==seq:break
   with socket.create_connection(('127.0.0.1',tcp[3]),timeout=3) as game:
    sender,kind,cmd,pid,error,payload=receive(game);assert (sender,kind,cmd)==(0x28,1,0x1031)
    digits=f'{int(ticket[:4])+pid:05d}{int(ticket[4:])+pid:05d}'.encode()
    frame(game,0x82,2,cmd,pid,digits)
    def record():
     n=readn(game,1);return n+readn(game,n[0]-1)
    assert record()==bytes([3,0x90,0])
    queued=record();assert queued[1]==0x40 and queued[4:]==early,'early native data lost during handoff'
    game.sendall(bytes([3,0x91,0]))
    assert record()==bytes([3,0x90,1])
    probes=[record() for _ in range(8)]
    assert [p[2] for p in probes]==list(range(8))
    echoes=[bytes([8,0x31,0])+p[2:] for p in probes]
    game.sendall(echoes[0]*8)
    game.settimeout(.1)
    try:unexpected=game.recv(1);raise AssertionError(('duplicate probes completed phase',unexpected))
    except socket.timeout:pass
    finally:game.settimeout(3)
    game.sendall(b''.join(echoes[1:]))
    assert record()==bytes([3,0x90,2])
    if pause_stun is not None and label == 'File2-US':
     # Keep both authenticated game sockets open while the RFC probe fails.
     pause_stun()
    native_body=bytes(range(36))
    game.sendall(bytes([41,0x71])+struct.pack('<H',0x1000|36)+native_body+b'\xff')
    delivered=receive_snap(host,udp[2])
    assert delivered['cmd']==15 and delivered['sid']==guest_sid and delivered['payload']==struct.pack('<H',0x1000|36)+native_body
    host.sendto(struct.pack('>HBBIII',0x4010,0,0,sid,0,delivered['seq'])+bytes.fromhex('ba476611'),('127.0.0.1',udp[2]))
    game.sendall(bytes([2,0xf1]))
    empty=receive_snap(host,udp[2]);assert empty['cmd']==15 and empty['flags']==0x2000 and empty['payload']==b'\0\x10'
    # Reverse conversion preserves the body and adds only measured transport padding.
    western_body=bytes(reversed(range(36)))
    seq+=1;payload=struct.pack('<H',36)+western_body
    host.sendto(struct.pack('>HBBIII',0xa000|(16+len(payload)),0,15,sid,seq,0)+payload+bytes.fromhex('ba476611'),('127.0.0.1',udp[2]))
    native=record();assert native[:4]==bytes([41,0x70,36,0]) and native[4:40]==western_body
    assert len(get('/state/file2')['rooms'][0]['members'])==2
   assert exchange(host,udp[2],0xa000,2,sid,seq+1)['cmd']==0x28
   assert not get('/state/file2')['rooms']
   outcomes.append({'variant':label,'actual_host_greeting_before_native6504':'PASS','western_rules_and_native_wait_timer':'PASS','announced_6910':'PASS','actual_guest_query_before_western_ack_04':'PASS','host_05_to_08_table':'PASS','native_seat_rules_session_endpoint':'PASS','no_score_words_in_empty_native_reply':'PASS','6002_scenario_transition':'PASS','lobby_disconnect_retains_seat':'PASS','original_client':False,'engine_envelope_roundtrip':'PASS with constructed opaque bodies','early_engine_body_retained':'PASS','distinct_probe_echoes_required':'PASS','gameplay_validated':False})
 return outcomes

if __name__=='__main__':
 import argparse
 parser = argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--native-controller', help='directory containing the NM native controller Python modules')
 parser.add_argument('--lobby-only', action='store_true', help='check live room information and exit without starting the scenario')
 args = parser.parse_args()
 main(partial(exercise,lobby_only=args.lobby_only), check_active_stun_outage=not args.lobby_only, native_controller=args.native_controller)
