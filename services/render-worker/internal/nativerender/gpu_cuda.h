#ifndef SYNTELLE_GPU_CUDA_H
#define SYNTELLE_GPU_CUDA_H
#include <stddef.h>
#include "gpu_types.h"
typedef struct GPUContext GPUContext;
typedef struct { float body_ms,compose_ms,convert_ms; double sync_ms; } GPUFrameTiming;
GPUContext *syntelle_gpu_open(const char *ptx,int *status);
int syntelle_gpu_upload(GPUContext*,int slot,const void*,size_t);
int syntelle_gpu_share(GPUContext*,int slot,const void*,size_t,const unsigned char digest[32]);
void syntelle_gpu_memory(size_t *shared_bytes,size_t *allocations,size_t *leases);
size_t syntelle_gpu_private_bytes(GPUContext*);
int syntelle_gpu_patch(GPUContext*,int x,int y,int w,int h,int source_width,const unsigned char*);
int syntelle_gpu_run(GPUContext*,const GPUParams*,unsigned char*,int stride,const GPUOutput*,GPUFrameTiming*);
int syntelle_gpu_update(GPUContext*,int kind,const void*,int source_width);
int syntelle_gpu_readback(GPUContext*,unsigned char*,int width,int height,int stride);
int syntelle_gpu_close(GPUContext*);
#endif
