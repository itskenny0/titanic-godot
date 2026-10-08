import cat.kenny.taoot.HdPackFolder;
import java.io.IOException;
import java.util.*;

public final class HdPackFolderTest {
 static void check(boolean ok,String message){if(!ok)throw new AssertionError(message);}
 static class Documents implements HdPackFolder.Documents {
  Map<String,List<HdPackFolder.Entry>> directories=new HashMap<>();
  int lists=0,reads=0,lastLimit=0;
  String lastDocument;
  public List<HdPackFolder.Entry> list(String id)throws IOException{
   lists++;if(!directories.containsKey(id))throw new IOException("Permission revoked");return directories.get(id);
  }
  public byte[] read(String id,int limit)throws IOException{
   reads++;lastDocument=id;lastLimit=limit;
   if(id.equals("unavailable"))throw new IOException("Card removed");
   return new byte[]{1,2,3};
  }
 }
 interface Attempt {void run()throws Exception;}
 static void fails(Attempt attempt,String label)throws Exception{
  try{attempt.run();}catch(IOException expected){return;}throw new AssertionError(label);
 }
 public static void main(String[] args)throws Exception{
  String key=String.join("",Collections.nCopies(64,"a"));
  Documents d=new Documents();
  d.directories.put("root",Arrays.asList(new HdPackFolder.Entry("MANIFEST.JSON","manifest-id",false),new HdPackFolder.Entry("Images","images-id",true)));
  List<HdPackFolder.Entry> images=new ArrayList<>();
  images.add(new HdPackFolder.Entry(key.toUpperCase()+".WEBP","opaque-provider-id",false));
  images.add(new HdPackFolder.Entry("notes.txt","ignored",false));
  d.directories.put("images-id",images);
  HdPackFolder pack=new HdPackFolder(d,"root");
  check(d.lists==2&&d.reads==0,"Index names without copying or reading image data");
  pack.read("manifest.json");check(d.lastLimit==16*1024*1024,"Manifest read bound");
  for(int i=0;i<3;i++)pack.read("images/"+key+".webp");
  check(d.lists==2&&d.lastDocument.equals("opaque-provider-id")&&d.lastLimit==8*1024*1024,"Direct indexed reads, no directory scans or assumed document IDs");
  fails(()->pack.read("images/../manifest.json"),"Reject traversal");
  fails(()->pack.read("images/"+key+".png"),"Missing file fails");
  images.add(new HdPackFolder.Entry(key+".webp","duplicate",false));
  fails(()->new HdPackFolder(d,"root"),"Reject ambiguous case variants");
  fails(()->new HdPackFolder(d,"missing"),"Revoked folder permission fails");
  List<HdPackFolder.Entry> siblings=Arrays.asList(new HdPackFolder.Entry("cd1.iso","disc1",false),new HdPackFolder.Entry("HDPACK","pack-root",true));
  check(HdPackFolder.find(siblings,"hdpack").id.equals("pack-root"),"Find pack beside ISOs in any case");
  check(HdPackFolder.find(siblings,"absent")==null,"Absent pack allowed");
  fails(()->HdPackFolder.find(Arrays.asList(new HdPackFolder.Entry("hdpack","a",true),new HdPackFolder.Entry("HDPACK","b",true)),"hdpack"),"Reject ambiguous pack folder");
  d.directories.put("missing-manifest",Arrays.asList(new HdPackFolder.Entry("images","images-id",true)));
  fails(()->new HdPackFolder(d,"missing-manifest"),"Reject incomplete folder");
  System.out.println("ANDROID HD FOLDER PASS");
 }
}
