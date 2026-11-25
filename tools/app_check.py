"""Independent Capcom APP client; reconstructs native frames, never runs a PS2 game."""
import socket,struct,hashlib

def field(data,seq):
 masked=bytes(v^b'MINAMIOH'[i&7]^((seq+i)&255) for i,v in enumerate(data))
 return struct.pack('>HH',len(data)+2,(sum(data)+seq*257)&0x7fff)+masked

def decode(data,seq):
 assert len(data)>=4
 size,check=struct.unpack_from('>HH',data);size+=2;assert 4<=size<=len(data)
 plain=bytes(v^b'MINAMIOH'[i&7]^((seq+i)&255) for i,v in enumerate(data[4:size]))
 assert check==(sum(plain)+seq*257)&0x7fff
 return plain,size

def readn(peer,n):
 data=b''
 while len(data)<n:
  chunk=peer.recv(n-len(data));assert chunk,'truncated APP frame';data+=chunk
 return data

def receive(peer):
 h=readn(peer,8);length,direction,cmd,seq,err,tail=struct.unpack('>HBHBBB',h)
 assert tail==255
 return direction,cmd,seq,err,readn(peer,length)

def send(peer,direction,cmd,seq,payload=b''):
 peer.sendall(struct.pack('>HBHBBB',len(payload),direction,cmd,seq,0,255)+payload)

def check_app(port,name,build):
 with socket.create_connection(('127.0.0.1',port),timeout=3) as peer:
  assert receive(peer)==(1,0x1002,0,0,b'\0\0')
  seq=3;send(peer,2,0x1002,seq,field(name.encode(),seq)+bytes([build,1])+field(b'0404141723',seq)+bytes(4))
  assert receive(peer)==(16,0x6001,0,0,b'\0'),'server must not offer a client patch'
  def request(cmd,payload=b''):
   nonlocal seq
   seq=(seq+1)&255;data=payload(seq) if callable(payload) else payload
   send(peer,1,cmd,seq,data);direction,answer,number,error,content=receive(peer)
   assert (direction,answer,number,error)==(2,cmd,seq,0),(cmd,answer,error)
   return content
  page=b'01/TOP_INFOR.HTM'
  info=request(0x6101,lambda q:bytes(4)+field(page,q));assert info[:5]==b'\1'+bytes(4)
  named,used=decode(info[5:],seq);assert named==page
  length=struct.unpack_from('>I',info,5+used)[0];assert length>0
  chunk=request(0x6102,lambda q:field(page,q)+struct.pack('>IH',0,722));named,used=decode(chunk,seq);content,_=decode(chunk[used+4:],seq);assert named==page and len(content)==length and content.endswith(b'\0')
  if build!=1:
   for cmd in [0x6110,0x6220,0x6210,0x1007,0x6103,0x6212]:request(cmd)
  listing=request(0x6201 if build==1 else 0x6203)
  assert listing[0]==1;token,used=decode(listing[1:],seq);count=struct.unpack_from('>H',listing,1+used)[0]
  sizes=struct.unpack_from('>'+str(count)+'I',listing,3+used);files=[]
  for i,size in enumerate(sizes):
   downloaded=b''
   while len(downloaded)<size:
    off=len(downloaded);reply=request(0x6202 if build==1 else 0x6204,struct.pack('>HIH',i,off,722));assert reply[:6]==struct.pack('>HI',i,off)
    data,used=decode(reply[6:],seq);assert data and len(data)<=722;downloaded+=data
   assert len(downloaded)==size;files.append(downloaded)
  if build!=1:
   assert token==b'OPENSNAP-'+hashlib.sha1(files[0]).hexdigest()[:20].encode()
   assert request(0x6501,bytes(4))[0]==1
   worlds=request(0x6503,b'\0\0\0\1');assert worlds[:5]==b'\0\1\0\0\1'
   host,used=decode(worlds[5:],seq);assert host==b'127.0.0.1'
   lands=request(0x6504,lambda q:b'\0\0\0\1'+field(host,q));key,used=decode(lands[5:],seq);assert key==b'area'
   request(0x6502,lambda q:bytes(4)+field(host,q))
   request(0x6510,lambda q:b'\1'+field(key,q)+field(host,q))
   for cmd in [0x6211,0x6213]:request(cmd)
  request(0x6401)
  # Deliberately opaque, unattributed test record: ACK only after durable archive.
  request(0x6301,b'opaque-fixture-report')
  request(0x1004)
  return {'build':build,'native_open_and_no_patch':'PASS','information_page':'PASS','data_download':'PASS','file_sizes':list(sizes),'world_and_land': 'PASS' if build!=1 else 'not applicable','opaque_report_archive_reply':'PASS','complete':'PASS'}
