/* GPL-3.0. Indexed, read-only access to a selected HD folder. */
package cat.kenny.taoot;

import java.io.IOException;
import java.util.*;

public final class HdPackFolder {
 public interface Documents {
  List<Entry> list(String directory) throws IOException;
  byte[] read(String document,int limit) throws IOException;
 }
 public static final class Entry {
  public final String name,id;
  public final boolean directory;
  public Entry(String name,String id,boolean directory){this.name=name;this.id=id;this.directory=directory;}
 }
 private final Documents documents;
 private final Map<String,String> files=new HashMap<>();
 public HdPackFolder(Documents documents,String root)throws IOException{
  this.documents=documents;
  List<Entry> children=documents.list(root);
  Entry manifest=find(children,"manifest.json"),images=find(children,"images");
  if(manifest==null||manifest.directory||images==null||!images.directory)
   throw new IOException("Choose an hdpack folder containing manifest.json and images");
  files.put("manifest.json",manifest.id);
  for(Entry e:documents.list(images.id)){
   if(e.directory||!e.name.matches("[0-9a-fA-F]{64}\\.(?i:png|webp)"))continue;
   String name="images/"+e.name.toLowerCase(Locale.ROOT);
   if(files.put(name,e.id)!=null)throw new IOException("Conflicting HD image: "+e.name);
   if(files.size()>100001)throw new IOException("Too many HD images");
  }
 }
 public static Entry find(List<Entry> entries,String name)throws IOException{
  Entry result=null;
  for(Entry entry:entries)if(entry.name.equalsIgnoreCase(name)){
   if(result!=null)throw new IOException("Conflicting folder entries: "+name);
   result=entry;
  }
  return result;
 }
 public byte[] read(String path)throws IOException{
  String id=files.get(path);
  if(id==null)throw new IOException("Missing HD image: "+path);
  return documents.read(id,path.equals("manifest.json")?16*1024*1024:8*1024*1024);
 }
}
