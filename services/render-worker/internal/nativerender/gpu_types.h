#ifndef SYNTELLE_GPU_TYPES_H
#define SYNTELLE_GPU_TYPES_H
typedef struct { double x,y,z,u,v,q; } GPUVertex;
typedef struct { GPUVertex a,b,c; double inv; int flags[6]; } GPUTriangle;
typedef struct { int a,b; double f; int valid,pad; } GPUAxis;
typedef struct {
    int bw,bh,sw,sh,outw,outh,tiles_x,triangle_count;
    int ex,ey,ew,eh,fw,fh,mw,mh;
    double smile,brows,units;
} GPUParams;
typedef struct { double p[6],from[6],to[6],area; int flags[4]; } GPUOralTriangle;
typedef struct { int dims[16]; double values[60]; } GPUOralParams;
typedef struct { int dims[12]; double values[8]; } GPUEyeParams;
typedef struct { int dims[12]; double values[4]; } GPUSkinParams;
typedef struct { int dims[16]; double values[104]; } GPUReferenceParams;
typedef struct { unsigned long long context,planes[3]; int strides[3],width,height,marker; } GPUOutput;
#endif
