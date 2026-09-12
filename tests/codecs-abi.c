/* Exercise the exported Go ABI in cross-compiled PortMaster libraries. */
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
int main(int argc,char **argv){
 if(argc!=2)return 2;
 void *lib=dlopen(argv[1],RTLD_NOW);
 if(!lib){fprintf(stderr,"%s\n",dlerror());return 1;}
 int64_t (*frame)(uint8_t*,int64_t,uint8_t*,uint8_t*,int64_t,int,char*,int64_t)=dlsym(lib,"taoot_decode_frame");
 int (*pcm)(float*,int64_t,uint8_t*,double,double)=dlsym(lib,"taoot_stereo_pcm");
 if(!frame||!pcm)return 1;
 uint8_t data[]={2,0,2,0,4,1,2,4,3,4},pixels[6]={99,0,0,0,0,99},z[6]={99,0,0,0,0,99};char error[256]={0};
 if(frame(data,sizeof(data),pixels+1,z+1,4,0,error,sizeof(error))!=-1||memcmp(pixels,(uint8_t[]){99,1,2,3,4,99},6)||z[0]!=99||z[5]!=99){fprintf(stderr,"Frame ABI mismatch: %s\n",error);return 1;}
 float samples[]={-1,.5,1};uint8_t out[14]={99};out[13]=99;
 uint8_t expected[]={99,1,128,0,0,0,64,0,0,255,127,0,0,99};
 if(pcm(samples,3,out+1,1,-1)||memcmp(out,expected,sizeof(out))){fputs("PCM ABI mismatch\n",stderr);return 1;}
 if(frame(data,sizeof(data),pixels+1,z+1,1,0,error,sizeof(error))!=-2){fputs("Short buffer accepted\n",stderr);return 1;}
 puts("CODEC ABI PASS");return 0;
}
