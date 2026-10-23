//go:build nativegpu && cgo && linux && amd64

#include "gpu_cuda.h"
#include <ffnvcodec/dynlink_cuda.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include <pthread.h>

static struct {
    void *library;
    int status;
    tcuInit *init;
    tcuDeviceGet *device;
    tcuDevicePrimaryCtxRetain *retain;
    tcuDevicePrimaryCtxRelease *release;
    tcuDevicePrimaryCtxGetState *get_state;
    tcuDevicePrimaryCtxSetFlags *set_flags;
    tcuCtxPushCurrent_v2 *push;
    tcuCtxPopCurrent_v2 *pop;
    tcuModuleLoadData *module_load;
    tcuModuleUnload *module_free;
    tcuModuleGetFunction *function;
    tcuMemAlloc_v2 *alloc;
    tcuMemFree_v2 *free;
    tcuMemcpyHtoD_v2 *upload;
    tcuMemcpy2D_v2 *copy2d;
    tcuLaunchKernel *launch;
    tcuStreamCreate *stream_create;
    tcuStreamDestroy_v2 *stream_destroy;
    tcuStreamSynchronize *stream_sync;
} driver;
static pthread_once_t driver_once=PTHREAD_ONCE_INIT;
static pthread_mutex_t primary_context_lock=PTHREAD_MUTEX_INITIALIZER;
static void load_driver(void) {
    driver.status=-1000;
    driver.library=dlopen("libcuda.so.1",RTLD_NOW|RTLD_LOCAL);
    if(!driver.library) return;
#define LOAD(member,name) do { *(void**)(&driver.member)=dlsym(driver.library,name); if(!driver.member) return; } while(0)
    LOAD(init,"cuInit"); LOAD(device,"cuDeviceGet");
    LOAD(retain,"cuDevicePrimaryCtxRetain"); LOAD(release,"cuDevicePrimaryCtxRelease_v2");
    LOAD(get_state,"cuDevicePrimaryCtxGetState"); LOAD(set_flags,"cuDevicePrimaryCtxSetFlags_v2");
    LOAD(push,"cuCtxPushCurrent_v2"); LOAD(pop,"cuCtxPopCurrent_v2");
    LOAD(module_load,"cuModuleLoadData"); LOAD(module_free,"cuModuleUnload"); LOAD(function,"cuModuleGetFunction");
    LOAD(alloc,"cuMemAlloc_v2"); LOAD(free,"cuMemFree_v2"); LOAD(upload,"cuMemcpyHtoD_v2");
    LOAD(copy2d,"cuMemcpy2D_v2"); LOAD(launch,"cuLaunchKernel");
    LOAD(stream_create,"cuStreamCreate"); LOAD(stream_destroy,"cuStreamDestroy_v2"); LOAD(stream_sync,"cuStreamSynchronize");
#undef LOAD
    driver.status=driver.init(0);
}
typedef struct SharedBuffer {
    CUcontext context;
    unsigned char digest[32];
    CUdeviceptr pointer;
    size_t bytes,refs;
    struct SharedBuffer *next;
} SharedBuffer;
static pthread_mutex_t shared_lock=PTHREAD_MUTEX_INITIALIZER;
static SharedBuffer *shared_buffers;
struct GPUContext {
    CUcontext context;
    CUdevice device;
    CUmodule module;
    CUfunction body,compose,oral,eyes,skin,reference,convert;
    CUstream stream;
    GPUSkinParams skin_params;
    CUdeviceptr buffers[24];
    size_t capacity[24];
    SharedBuffer *shared[24];
};
_Static_assert(sizeof(GPUVertex)==48,"vertex ABI");
_Static_assert(sizeof(GPUTriangle)==176,"triangle ABI");
_Static_assert(sizeof(GPUAxis)==24,"axis ABI");
_Static_assert(sizeof(GPUParams)==88,"parameter ABI");

static int reserve(GPUContext *g,int slot,size_t n) {
    if(slot<0 || slot>=24 || n>128*1024*1024) return -1001;
    // A borrowed immutable allocation must never become a writable target.
    if(g->shared[slot]) return -1001;
    if(g->capacity[slot]>=n) return 0;
    CUdeviceptr replacement=0;
    int status=driver.alloc(&replacement,n);
    if(status) return status;
    if(g->buffers[slot]) {
        status=driver.free(g->buffers[slot]);
        if(status) { driver.free(replacement); return status; }
    }
    g->buffers[slot]=replacement; g->capacity[slot]=n;
    return 0;
}

GPUContext *syntelle_gpu_open(const char *ptx,int *status) {
    pthread_once(&driver_once,load_driver);
    *status=driver.status;
    if(*status) return NULL;
    GPUContext *g=calloc(1,sizeof(*g));
    if(!g) { *status=-1002; return NULL; }
    *status=driver.device(&g->device,0);
    if(*status) { free(g); return NULL; }
    // Default AUTO may busy-spin on synchronous GPU completion. A sleeping
    // wait releases CPU while retaining the same kernel/pixel calculations.
    unsigned int flags=0; int active=0;
    pthread_mutex_lock(&primary_context_lock);
    *status=driver.get_state(g->device,&flags,&active);
    if(!*status && !active) *status=driver.set_flags(g->device,CU_CTX_SCHED_BLOCKING_SYNC);
    if(!*status) *status=driver.retain(&g->context,g->device);
    pthread_mutex_unlock(&primary_context_lock);
    if(*status) { free(g); return NULL; }
    *status=driver.push(g->context);
    if(*status) { driver.release(g->device); free(g); return NULL; }
    *status=driver.module_load(&g->module,ptx);
    if(!*status) *status=driver.function(&g->body,g->module,"syntelle_body");
    if(!*status) *status=driver.function(&g->compose,g->module,"syntelle_compose");
    if(!*status) *status=driver.function(&g->oral,g->module,"syntelle_oral");
    if(!*status) *status=driver.function(&g->eyes,g->module,"syntelle_eyes");
    if(!*status) *status=driver.function(&g->skin,g->module,"syntelle_skin");
    if(!*status) *status=driver.function(&g->reference,g->module,"syntelle_reference");
    if(!*status) *status=driver.function(&g->convert,g->module,"syntelle_i420");
    if(!*status) *status=driver.stream_create(&g->stream,CU_STREAM_NON_BLOCKING);
    CUcontext previous;
    int pop=driver.pop(&previous);
    if(!*status) *status=pop;
    if(*status) { syntelle_gpu_close(g); return NULL; }
    return g;
}

int syntelle_gpu_upload(GPUContext *g,int slot,const void *source,size_t n) {
    if(!g || !source || !n) return -1001;
    int status=driver.push(g->context);
    if(status) return status;
    status=reserve(g,slot,n);
    if(!status) status=driver.upload(g->buffers[slot],source,n);
    CUcontext previous; int pop=driver.pop(&previous);
    return status?status:pop;
}

int syntelle_gpu_share(GPUContext *g,int slot,const void *source,size_t n,const unsigned char digest[32]) {
    if(!g || !digest || !n || n>128*1024*1024 || slot<0 || slot>=24 || g->buffers[slot]) return -1001;
    // Only read-only kernel inputs qualify. Subject, patches, projected geometry,
    // bins, backgrounds and output/encoder surfaces remain session-owned.
    if(slot!=1 && slot!=2 && slot!=4 && slot!=5 && slot!=11 && slot!=14 &&
       slot!=16 && slot!=17 && slot!=18 && slot!=19 && slot!=20 && slot!=21 && slot!=22) return -1001;
    int status=driver.push(g->context);if(status)return status;
    pthread_mutex_lock(&shared_lock);
    SharedBuffer *entry=shared_buffers;
    while(entry && (entry->context!=g->context || entry->bytes!=n || memcmp(entry->digest,digest,32))) entry=entry->next;
    if(!entry && !source) status=-1003; // Lookup miss: caller may build/upload once.
    if(!entry && source) {
        entry=calloc(1,sizeof(*entry));
        if(!entry) status=-1002;
        if(!status) status=driver.alloc(&entry->pointer,n);
        if(!status) status=driver.upload(entry->pointer,source,n);
        if(status) { if(entry && entry->pointer)driver.free(entry->pointer);free(entry);entry=NULL; }
        else {entry->context=g->context;entry->bytes=n;memcpy(entry->digest,digest,32);entry->next=shared_buffers;shared_buffers=entry;}
    }
    if(!status) {entry->refs++;g->shared[slot]=entry;g->buffers[slot]=entry->pointer;g->capacity[slot]=n;}
    pthread_mutex_unlock(&shared_lock);
    CUcontext previous;int pop=driver.pop(&previous);return status?status:pop;
}

void syntelle_gpu_memory(size_t *bytes,size_t *allocations,size_t *leases) {
    *bytes=*allocations=*leases=0;
    pthread_mutex_lock(&shared_lock);
    for(SharedBuffer *e=shared_buffers;e;e=e->next){*bytes+=e->bytes;(*allocations)++;*leases+=e->refs;}
    pthread_mutex_unlock(&shared_lock);
}

size_t syntelle_gpu_private_bytes(GPUContext *g) {
    size_t bytes=0;
    if(g)for(int i=0;i<24;i++)if(!g->shared[i])bytes+=g->capacity[i];
    return bytes;
}

// Caller has synchronized its own stream and has the primary context current.
static int release_shared(SharedBuffer *entry,int context_current) {
    int status=0;
    pthread_mutex_lock(&shared_lock);
    if(--entry->refs==0) {
        SharedBuffer **link=&shared_buffers;
        while(*link!=entry)link=&(*link)->next;
        *link=entry->next;
        if(context_current)status=driver.free(entry->pointer);
        free(entry);
    }
    pthread_mutex_unlock(&shared_lock);
    return status;
}

int syntelle_gpu_patch(GPUContext *g,int x,int y,int w,int h,int sw,const unsigned char *source) {
    if(!g || !source || x<0 || y<0 || w<1 || h<1 || sw<1 || x+w>sw ||
       ((size_t)(y+h-1)*sw+x+w)*4>g->capacity[0]) return -1001;
    int status=driver.push(g->context); if(status) return status;
    CUDA_MEMCPY2D copy={0};
    copy.srcMemoryType=CU_MEMORYTYPE_HOST; copy.srcHost=source; copy.srcPitch=w*4;
    copy.dstMemoryType=CU_MEMORYTYPE_DEVICE; copy.dstDevice=g->buffers[0]; copy.dstPitch=sw*4;
    copy.dstXInBytes=x*4; copy.dstY=y; copy.WidthInBytes=w*4; copy.Height=h;
    status=driver.copy2d(&copy);
    CUcontext previous; int pop=driver.pop(&previous);
    return status?status:pop;
}

int syntelle_gpu_run(GPUContext *g,const GPUParams *p,unsigned char *out,int stride,const GPUOutput *target) {
    if(!g || (!out&&!target) || p->bw<1 || p->bw>1024 || p->bh<1 || p->bh>1024 ||
       (p->outw!=720 && p->outw!=1440) || p->outh!=960 || (out&&stride<p->outw*4)) return -1001;
    if(target && (target->context!=(uintptr_t)g->context || target->width!=p->outw || target->height!=p->outh ||
       !target->planes[0] || !target->planes[1] || !target->planes[2] || target->strides[0]<p->outw ||
       target->strides[1]<p->outw/2 || target->strides[2]<p->outw/2)) return -1001;
    int status=driver.push(g->context); if(status) return status;
    status=reserve(g,9,(size_t)p->bw*p->bh*4);
    if(!status) status=reserve(g,10,(size_t)p->outw*p->outh*4);
    if(!status) {
        void *args[]={ (void*)p,&g->buffers[0],&g->buffers[1],&g->buffers[2],&g->buffers[6],&g->buffers[7],&g->buffers[8],&g->buffers[9] };
        status=driver.launch(g->body,(p->bw*p->bh+127)/128,1,1,128,1,1,0,g->stream,args,NULL);
    }
    if(!status) {
        void *args[]={ (void*)p,&g->buffers[9],&g->buffers[3],&g->buffers[4],&g->buffers[5],&g->buffers[10] };
        status=driver.launch(g->compose,(720*960+127)/128,1,1,128,1,1,0,g->stream,args,NULL);
    }
    if(!status && target) {
        void *args[]={(void*)target,&g->buffers[10]};
        status=driver.launch(g->convert,(p->outw*p->outh+127)/128,1,1,128,1,1,0,g->stream,args,NULL);
    }
    if(!status) status=driver.stream_sync(g->stream);
    if(!status && out) {
        CUDA_MEMCPY2D copy={0};
        copy.srcMemoryType=CU_MEMORYTYPE_DEVICE; copy.srcDevice=g->buffers[10]; copy.srcPitch=p->outw*4;
        copy.dstMemoryType=CU_MEMORYTYPE_HOST; copy.dstHost=out; copy.dstPitch=stride;
        copy.WidthInBytes=p->outw*4; copy.Height=p->outh;
        status=driver.copy2d(&copy);
    }
    CUcontext previous; int pop=driver.pop(&previous);
    return status?status:pop;
}

int syntelle_gpu_readback(GPUContext *g,unsigned char *out,int width,int height,int stride) {
    if(!g || !out || (width!=720&&width!=1440) || height!=960 || stride<width*4 || g->capacity[10]<(size_t)width*height*4) return -1001;
    int status=driver.push(g->context);if(status)return status;
    status=driver.stream_sync(g->stream);
    if(!status){CUDA_MEMCPY2D copy={0};copy.srcMemoryType=CU_MEMORYTYPE_DEVICE;copy.srcDevice=g->buffers[10];copy.srcPitch=width*4;
        copy.dstMemoryType=CU_MEMORYTYPE_HOST;copy.dstHost=out;copy.dstPitch=stride;copy.WidthInBytes=width*4;copy.Height=height;status=driver.copy2d(&copy);}
    CUcontext previous;int pop=driver.pop(&previous);return status?status:pop;
}

int syntelle_gpu_update(GPUContext *g,int kind,const void *params,int sw) {
    if(!g || !params || sw<1 || sw>4096)return -1001;
    int status=driver.push(g->context);if(status)return status;
    if(kind==1){
        const GPUSkinParams *p=params;g->skin_params=*p;
        status=reserve(g,15,(size_t)p->dims[2]*p->dims[3]*4);
        if(!status){void *args[]={(void*)p,&g->buffers[0],&g->buffers[14],&g->buffers[15],&g->buffers[16],&g->buffers[17],&sw};
            status=driver.launch(g->skin,(p->dims[2]*p->dims[3]+127)/128,1,1,128,1,1,0,g->stream,args,NULL);}
    }else if(kind==2){
        const GPUOralParams *p=params;
        void *args[]={(void*)p,&g->buffers[12],&g->buffers[11],&g->buffers[0],&g->buffers[14],&g->buffers[15],&sw,&g->skin_params};
        status=driver.launch(g->oral,(p->dims[0]*p->dims[1]+127)/128,1,1,128,1,1,0,g->stream,args,NULL);
    }else if(kind==3){
        const GPUEyeParams *p=params;
        void *args[]={(void*)p,&g->buffers[18],&g->buffers[19],&g->buffers[20],&g->buffers[0],&g->buffers[14],&g->buffers[15],&sw,&g->skin_params};
        status=driver.launch(g->eyes,(p->dims[0]*p->dims[1]+127)/128,1,1,128,1,1,0,g->stream,args,NULL);
    }else if(kind==4){
        const GPUReferenceParams *p=params;
        void *args[]={(void*)p,&g->buffers[21],&g->buffers[22],&g->buffers[18],&g->buffers[19],&g->buffers[20],&g->buffers[11],&g->buffers[0],&g->buffers[14],&sw};
        status=driver.launch(g->reference,(p->dims[2]*p->dims[3]+127)/128,1,1,128,1,1,0,g->stream,args,NULL);
    }else status=-1001;
    CUcontext previous;int pop=driver.pop(&previous);return status?status:pop;
}

int syntelle_gpu_close(GPUContext *g) {
    if(!g) return 0;
    int status=driver.push(g->context);
    if(!status) {
        int synced=driver.stream_sync(g->stream);if(!status)status=synced;
        for(int i=0;i<24;i++) if(g->buffers[i]) { int e=g->shared[i]?release_shared(g->shared[i],1):driver.free(g->buffers[i]); if(!status) status=e; }
        if(g->stream){int e=driver.stream_destroy(g->stream);if(!status)status=e;}
        if(g->module) { int e=driver.module_free(g->module); if(!status) status=e; }
        CUcontext previous; int e=driver.pop(&previous); if(!status) status=e;
    } else {
        // A lost context cannot be used for CUDA frees. Still retire CPU leases
        // so a later primary context cannot acquire dangling cached pointers.
        for(int i=0;i<24;i++)if(g->shared[i])release_shared(g->shared[i],0);
    }
    int released=driver.release(g->device); if(!status) status=released;
    free(g); return status;
}
