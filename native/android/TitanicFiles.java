/* GPL-3.0. Android document access for the Godot player. */
package cat.kenny.taoot;

import android.app.Activity;
import android.content.Intent;
import android.database.Cursor;
import android.net.Uri;
import android.provider.DocumentsContract;
import org.godotengine.godot.Godot;
import org.godotengine.godot.plugin.GodotPlugin;
import org.godotengine.godot.plugin.SignalInfo;
import org.godotengine.godot.plugin.UsedByGodot;
import java.io.*;
import java.util.*;

public final class TitanicFiles extends GodotPlugin {
 private static final int PICK_GAME=1912,PICK_MODS=1913,PICK_SAVE=1914,EXPORT_SAVE=1915,PICK_PATCH=1916,PICK_HD=1917;
 private String destination,exportSource;
 private volatile String gameTree="";
 @UsedByGodot public String selected_game_tree(){return gameTree;}
 private volatile boolean busy=false;
 private final Map<String,HdPackFolder> hdFolders=new java.util.concurrent.ConcurrentHashMap<>();
 private final HdPackFolder.Documents hdDocuments=new HdPackFolder.Documents(){
  public List<HdPackFolder.Entry> list(String directory)throws IOException{
   Uri uri=Uri.parse(directory);
   Uri children=DocumentsContract.buildChildDocumentsUriUsingTree(uri,DocumentsContract.getDocumentId(uri));
   List<HdPackFolder.Entry> result=new ArrayList<>();
   try(Cursor c=getActivity().getContentResolver().query(children,new String[]{DocumentsContract.Document.COLUMN_DOCUMENT_ID,DocumentsContract.Document.COLUMN_DISPLAY_NAME,DocumentsContract.Document.COLUMN_MIME_TYPE},null,null,null)){
    if(c==null)throw new IOException("Cannot read the HD folder. Select it again to grant access.");
    while(c.moveToNext()){
     result.add(new HdPackFolder.Entry(c.getString(1),DocumentsContract.buildDocumentUriUsingTree(uri,c.getString(0)).toString(),DocumentsContract.Document.MIME_TYPE_DIR.equals(c.getString(2))));
     if(result.size()>100002)throw new IOException("Too many entries in HD folder");
    }
   }
   return result;
  }
  public byte[] read(String document,int limit)throws IOException{
   try(InputStream in=getActivity().getContentResolver().openInputStream(Uri.parse(document));ByteArrayOutputStream out=new ByteArrayOutputStream()){
    transfer(in,out,limit);return out.toByteArray();
   }
  }
 };
 private String hdRoot(Uri tree){return DocumentsContract.buildDocumentUriUsingTree(tree,DocumentsContract.getTreeDocumentId(tree)).toString();}
 @UsedByGodot public String prepare_hd_folder(String root){
  try{
   if(!hdFolders.containsKey(root)){
    HdPackFolder folder=new HdPackFolder(hdDocuments,root);
    org.json.JSONObject manifest=new org.json.JSONObject(new String(folder.read("manifest.json"),java.nio.charset.StandardCharsets.UTF_8));
    int count=manifest.getJSONObject("images").length();
    if((manifest.getInt("version")!=1&&manifest.getInt("version")!=2)||manifest.getInt("scale")!=2||count<1||count>100000)throw new IOException("Unsupported HD manifest");
    hdFolders.put(root,folder);
    android.util.Log.i("TitanicFiles","HD folder ready: "+count+" images, read in place");
   }
   return "";
  }catch(Exception e){return e.getMessage()==null?e.toString():e.getMessage();}
 }
 @UsedByGodot public byte[] read_hd_file(String path){
  int split=path.lastIndexOf("/images/");
  if(split<0&&path.endsWith("/manifest.json"))split=path.length()-14;
  if(split<0)return new byte[0];
  String root=path.substring(0,split),relative=path.substring(split+1);
  try{
   HdPackFolder folder=hdFolders.get(root);
   if(folder==null)throw new IOException("HD folder is not open");
   return folder.read(relative);
  }catch(Exception e){android.util.Log.w("TitanicFiles","HD read failed: "+e.getMessage());return new byte[0];}
 }
 private void rememberTree(Uri uri){getActivity().getContentResolver().takePersistableUriPermission(uri,Intent.FLAG_GRANT_READ_URI_PERMISSION);}
 @UsedByGodot public String find_hd_folder(String tree){
  try{
   HdPackFolder.Entry entry=HdPackFolder.find(hdDocuments.list(hdRoot(Uri.parse(tree))),"hdpack");
   return entry!=null&&entry.directory?entry.id:"";
  }catch(Exception e){android.util.Log.w("TitanicFiles","HD detection: "+e.getMessage());return "";}
 }
 private void detectHD(Uri uri){
  try{
   HdPackFolder.Entry entry=HdPackFolder.find(hdDocuments.list(hdRoot(uri)),"hdpack");
   if(entry==null||!entry.directory)return;
   rememberTree(uri);
   String error=prepare_hd_folder(entry.id);
   emitSignal("hd_detected",error.isEmpty()?entry.id:"",error);
  }catch(Exception e){emitSignal("hd_detected","",e.getMessage()==null?e.toString():e.getMessage());}
 }
 public TitanicFiles(Godot godot){super(godot);}
 @Override public String getPluginName(){return "TitanicFiles";}
 @Override public Set<SignalInfo> getPluginSignals(){return new HashSet<>(Arrays.asList(
  new SignalInfo("hd_folder_selected",String.class,String.class),
  new SignalInfo("hd_detected",String.class,String.class),
  new SignalInfo("import_finished",String.class,String.class),
  new SignalInfo("mod_import_finished",String.class,String.class),
  new SignalInfo("patch_import_finished",String.class,String.class),
  new SignalInfo("save_import_finished",String.class,String.class),
  new SignalInfo("export_finished",String.class,String.class)));}
 private String signal(int code){return code==PICK_HD?"hd_folder_selected":code==PICK_PATCH?"patch_import_finished":code==PICK_MODS?"mod_import_finished":code==PICK_SAVE?"save_import_finished":code==EXPORT_SAVE?"export_finished":"import_finished";}
 private void pick(int code,String target){
  if(busy)return;busy=true;destination=target;
  runOnUiThread(()->{
   Intent intent=new Intent((code==PICK_SAVE||code==PICK_PATCH)?Intent.ACTION_OPEN_DOCUMENT:Intent.ACTION_OPEN_DOCUMENT_TREE);
   if(code==PICK_SAVE||code==PICK_PATCH){intent.setType("*/*");intent.addCategory(Intent.CATEGORY_OPENABLE);}
   intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION|Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
   try{getActivity().startActivityForResult(intent,code);}catch(Exception e){busy=false;emitSignal(signal(code),"",e.toString());}
  });
 }
 @UsedByGodot public void choose_hd_folder(){pick(PICK_HD,"");}
 @UsedByGodot public void import_game(String target){pick(PICK_GAME,target);}
 @UsedByGodot public void import_mods(String target){pick(PICK_MODS,target);}
 @UsedByGodot public void import_save(String target){pick(PICK_SAVE,target);}
 @UsedByGodot public void import_patches(String target){pick(PICK_PATCH,target);}
 @UsedByGodot public void export_save(String source){
  if(busy)return;busy=true;exportSource=source;
  runOnUiThread(()->{Intent i=new Intent(Intent.ACTION_CREATE_DOCUMENT);i.setType("application/octet-stream");i.addCategory(Intent.CATEGORY_OPENABLE);i.putExtra(Intent.EXTRA_TITLE,new File(source).getName());
   try{getActivity().startActivityForResult(i,EXPORT_SAVE);}catch(Exception e){busy=false;emitSignal("export_finished","",e.toString());}});
 }
 @Override public void onMainActivityResult(int code,int result,Intent intent){
  if(code<PICK_GAME||code>PICK_HD)return;
  if(result!=Activity.RESULT_OK||intent==null||intent.getData()==null){busy=false;emitSignal(signal(code),"","");return;}
  final Uri uri=intent.getData();
  new Thread(()->{
   File staging=null;
   try{
    if(code==PICK_HD){
     rememberTree(uri);String root=hdRoot(uri),error=prepare_hd_folder(root);
     emitSignal(signal(code),error.isEmpty()?root:"",error);
    }
    else if(code==EXPORT_SAVE){try(InputStream in=new FileInputStream(exportSource);OutputStream out=getActivity().getContentResolver().openOutputStream(uri,"wt")){transfer(in,out);}emitSignal(signal(code),uri.toString(),"");}
    else {
     // A unique import directory keeps previous data and saves intact on failure.
     File parent=new File(destination,"Imports");if(!parent.exists()&&!parent.mkdirs())throw new IOException("Cannot create import folder");
     staging=new File(parent,UUID.randomUUID().toString());if(!staging.mkdir())throw new IOException("Cannot create import staging folder");
     if(code==PICK_SAVE||code==PICK_PATCH){File save=new File(staging,code==PICK_PATCH?"patches.zip":"imported.ti");try(InputStream in=getActivity().getContentResolver().openInputStream(uri);OutputStream out=new FileOutputStream(save)){transfer(in,out);}emitSignal(signal(code),save.getAbsolutePath(),"");}
     else {copyTree(uri,DocumentsContract.getTreeDocumentId(uri),staging,0,new long[]{0,0});android.util.Log.i("TitanicFiles","Folder import completed");if(code==PICK_GAME){gameTree=uri.toString();try{rememberTree(uri);}catch(Exception e){android.util.Log.w("TitanicFiles","Folder permission could not be retained",e);}detectHD(uri);}emitSignal(signal(code),staging.getAbsolutePath(),"");}
    }
   }catch(Exception e){if(staging!=null)removeTree(staging);emitSignal(signal(code),"",e.getMessage()==null?e.toString():e.getMessage());}
   finally{busy=false;}
  }, "TitanicImport").start();
 }
 private boolean discImage(String name){String n=name.toLowerCase(Locale.ROOT);return n.endsWith(".iso")&&(n.contains("cd1")||n.contains("cd2"));}
 private boolean gameFile(String name){String n=name.toLowerCase(Locale.ROOT);return discImage(name)||n.equals("bootfile")||n.matches(".*\\.(set|shp|stg|cst|pup|mov|trk|sfx|snd|11k)$");}
 private void copyTree(Uri tree,String id,File target,int depth,long[] count)throws IOException{
  if(depth>8)throw new IOException("Game folder is nested too deeply");
  Uri children=DocumentsContract.buildChildDocumentsUriUsingTree(tree,id);
  try(Cursor c=getActivity().getContentResolver().query(children,new String[]{DocumentsContract.Document.COLUMN_DOCUMENT_ID,DocumentsContract.Document.COLUMN_DISPLAY_NAME,DocumentsContract.Document.COLUMN_MIME_TYPE},null,null,null)){
   if(c==null)throw new IOException("Cannot list the selected folder");
   Set<String> names=new HashSet<>();
   while(c.moveToNext()){
    String childId=c.getString(0),name=c.getString(1);boolean dir=DocumentsContract.Document.MIME_TYPE_DIR.equals(c.getString(2));
    if(name.startsWith(".")||(dir&&name.equalsIgnoreCase("hdpack"))||(!dir&&!gameFile(name)))continue;
    if(name.contains("/")||name.contains("\\")||!names.add(name.toLowerCase(Locale.ROOT)))throw new IOException("Conflicting or invalid file name: "+name);
    File file=new File(target,name);
    if(dir){if(!file.mkdir())throw new IOException("Cannot create "+name);copyTree(tree,childId,file,depth+1,count);}
    else {
     if(++count[0]>2000)throw new IOException("Too many files; choose the game folder or its LOCAL folder");
     Uri document=DocumentsContract.buildDocumentUriUsingTree(tree,childId);
     try(InputStream in=getActivity().getContentResolver().openInputStream(document);OutputStream out=new FileOutputStream(file)){count[1]+=transfer(in,out,discImage(name)?1024L*1024*1024:512L*1024*1024);}
     if(count[1]>4L*1024*1024*1024)throw new IOException("Import exceeds 4 GiB");
    }
   }
  }
 }
 private long transfer(InputStream in,OutputStream out)throws IOException{
  return transfer(in,out,512L*1024*1024);
 }
 private long transfer(InputStream in,OutputStream out,long limit)throws IOException{
  if(in==null||out==null)throw new IOException("Cannot open document");byte[] b=new byte[131072];long total=0;int n;
  while((n=in.read(b))!=-1){total+=n;if(total>limit)throw new IOException("Unexpectedly large game file");out.write(b,0,n);}out.flush();return total;
 }
 private void removeTree(File f){File[] children=f.listFiles();if(children!=null)for(File child:children)removeTree(child);f.delete();}
}
