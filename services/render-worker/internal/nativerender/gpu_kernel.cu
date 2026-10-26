// Compile to portable PTX, sm_60+, with FP contraction disabled. Pixel ownership,
// double-precision sampling and byte quantization mirror the Go reference.
#include "gpu_types.h"
#define DEVICE __attribute__((device))
#define KERNEL extern "C" __attribute__((global))
struct Color { double v[4]; };
DEVICE unsigned int index_1d() {
    unsigned int t,b,n;
    asm("mov.u32 %0, %%tid.x;" : "=r"(t));
    asm("mov.u32 %0, %%ctaid.x;" : "=r"(b));
    asm("mov.u32 %0, %%ntid.x;" : "=r"(n));
    return b*n+t;
}
DEVICE double mixd(double a,double b,double f) { return a+(b-a)*f; }
DEVICE unsigned char byte_color(double v) {
    if(v<=0) return 0; if(v>=1) return 255;
    return (unsigned char)(v*255+.5);
}
DEVICE Color sample(const unsigned char *p,int w,int h,double x,double y) {
    x=x<0?0:(x>w-1?w-1:x); y=y<0?0:(y>h-1?h-1:y);
    int x0=(int)x,y0=(int)y,x1=x0+1<w?x0+1:w-1,y1=y0+1<h?y0+1:h-1;
    double fx=x-x0,fy=y-y0;
    int a=(y0*w+x0)*4,b=(y0*w+x1)*4,c=(y1*w+x0)*4,d=(y1*w+x1)*4;
    Color out;
    for(int k=0;k<4;k++) out.v[k]=mixd(mixd(p[a+k],p[b+k],fx),mixd(p[c+k],p[d+k],fx),fy)/255;
    return out;
}
DEVICE Color fragment(GPUParams p,const unsigned char *subject,const unsigned char *field,const unsigned char *material,const unsigned char *emotion,double u,double v) {
    bool emotional=false; for(int k=1;k<10;k++) if(p.emotion_weights[k]!=0) emotional=true;
    if(p.smile>0 || p.brows>0 || emotional) {
        double tx=u*p.sw-.5,ty=v*p.sh-.5;
        if((tx>=p.ex-.5 && tx<=p.ex+p.ew-.5 && ty>=p.ey-.5 && ty<=p.ey+p.eh-.5) || (emotional && tx>=p.emotion_dims[0]-.5 && tx<=p.emotion_dims[0]+p.emotion_dims[2]-.5 && ty>=p.emotion_dims[1]-.5 && ty<=p.emotion_dims[1]+p.emotion_dims[3]-.5)) {
            double x=tx,y=ty;
            for(int i=0;i<(emotional?p.emotion_dims[6]:5);i++) {
                double dx=0,dy=0;
                if(field && p.fw>0 && p.fh>0) {
                    double fu=(x-p.ex+.5)/p.ew,fv=(y-p.ey+.5)/p.eh;
                    if(fu>=0 && fu<=1 && fv>=0 && fv<=1) {
                        Color c=sample(field,p.fw,p.fh,fu*p.fw-.5,fv*p.fh-.5);
                        dx=(c.v[0]*255-128)*p.smile*p.units;
                        dy=((c.v[1]*255-128)*p.smile+(c.v[2]*255-128)*p.brows)*p.units;
                    }
                }
                if(emotional && emotion && p.emotion_dims[4]>0 && p.emotion_dims[5]>0) {
                    double eu=(x-p.emotion_dims[0]+.5)/p.emotion_dims[2],ev=(y-p.emotion_dims[1]+.5)/p.emotion_dims[3];
                    if(eu>=0 && eu<=1 && ev>=0 && ev<=1) {
                        double ex=eu*p.emotion_dims[4]-.5,ey=ev*p.emotion_dims[5]-.5;
                        ex=ex<0?0:(ex>p.emotion_dims[4]-1?p.emotion_dims[4]-1:ex);
                        ey=ey<0?0:(ey>p.emotion_dims[5]-1?p.emotion_dims[5]-1:ey);
                        for(int k=1;k<10;k++) if(p.emotion_weights[k]!=0) {
                            Color ec=sample(emotion,p.emotion_dims[4],p.emotion_dims[5]*10,ex,ey+k*p.emotion_dims[5]);
                            dx+=(ec.v[0]*255-128)*p.emotion_units*p.emotion_weights[k];
                            dy+=(ec.v[1]*255-128)*p.emotion_units*p.emotion_weights[k];
                        }
                    }
                }
                double nx=tx-dx,ny=ty-dy;
                if(nx==x && ny==y) break;
                x=nx; y=ny;
            }
            u=(x+.5)/p.sw; v=(y+.5)/p.sh;
        }
    }
    Color c=sample(subject,p.sw,p.sh,u*p.sw-.5,v*p.sh-.5);
    if(p.smile>0 && material && p.mw>0 && p.mh>0) {
        double x=(u*p.sw-p.ex)/p.ew,y=(v*p.sh-p.ey)/p.eh;
        if(x>=0 && x<=1 && y>=0 && y<=1) {
            Color ref=sample(material,p.mw,p.mh,x*p.ew-.5,y*p.eh-.5);
            for(int k=0;k<3;k++) c.v[k]=c.v[k]*(1-ref.v[3]*p.smile)+ref.v[k]*c.v[3]*p.smile;
        }
    }
    return c;
}
DEVICE double edge_gpu(GPUVertex a,GPUVertex b,double x,double y) { return (x-a.x)*(b.y-a.y)-(y-a.y)*(b.x-a.x); }

KERNEL void syntelle_body(GPUParams p,const unsigned char *subject,const unsigned char *field,const unsigned char *material,const unsigned char *emotion,
                    const GPUTriangle *triangles,const int *offsets,const int *ids,unsigned char *body) {
    unsigned int at=index_1d(); if(at>=(unsigned int)(p.bw*p.bh)) return;
    int x=at%p.bw,y=at/p.bw,tile=(y/16)*p.tiles_x+x/16;
    double px=x+.5,py=y+.5,depth=1e300;
    unsigned char previous[4]={0,0,0,0};
    for(int j=offsets[tile];j<offsets[tile+1];j++) {
        GPUTriangle t=triangles[ids[j]];
        double e0=edge_gpu(t.b,t.c,px,py),e1=edge_gpu(t.c,t.a,px,py),e2=edge_gpu(t.a,t.b,px,py);
        if(e0<0 || e1<0 || e2<0 || (e0==0 && !t.flags[0]) || (e1==0 && !t.flags[1]) || (e2==0 && !t.flags[2])) continue;
        double wa=e0*t.inv,wb=e1*t.inv,wc=e2*t.inv,z=wa*t.a.z+wb*t.b.z+wc*t.c.z;
        if(z>depth+1e-10) continue;
        double u,v;
        if(t.flags[3]) { u=px/p.bw; v=py/p.bh; }
        else {
            double inverse=1/(wa*t.a.q+wb*t.b.q+wc*t.c.q);
            u=(wa*t.a.u+wb*t.b.u+wc*t.c.u)*inverse;
            v=(wa*t.a.v+wb*t.b.v+wc*t.c.v)*inverse;
        }
        Color c=fragment(p,subject,field,material,emotion,u,v);
        if(t.flags[4]) {
            if(c.v[3]>=.995 || c.v[3]<=.001) continue;
            for(int k=0;k<4;k++) c.v[k]+=previous[k]/255.0*(1-c.v[3]);
        } else {
            if(c.v[3]<.995) continue;
            depth=z;
        }
        for(int k=0;k<4;k++) previous[k]=byte_color(c.v[k]);
    }
    for(int k=0;k<4;k++) body[at*4+k]=previous[k];
}

KERNEL void syntelle_compose(GPUParams p,const unsigned char *body,const unsigned char *background,
                       const GPUAxis *xs,const GPUAxis *ys,unsigned char *out) {
    unsigned int at=index_1d(); if(at>=720*960) return;
    int x=at%720,y=at/720,i=(y*p.outw+x)*4;
    GPUAxis xa=xs[x],ya=ys[y]; double values[4]={0,0,0,0};
    if(xa.valid && ya.valid) {
        int a=ya.a+xa.a,b=ya.a+xa.b,c=ya.b+xa.a,d=ya.b+xa.b;
        if(body[a+3]|body[b+3]|body[c+3]|body[d+3]) {
            for(int k=0;k<4;k++) values[k]=mixd(mixd(body[a+k],body[b+k],xa.f),mixd(body[c+k],body[d+k],xa.f),ya.f);
        }
    }
    if(p.outw==1440) {
        for(int k=0;k<3;k++) out[i+k]=(unsigned char)(values[k]+.5);
        unsigned char alpha=(unsigned char)(values[3]+.5);
        for(int k=0;k<3;k++) out[i+720*4+k]=alpha;
        out[i+720*4+3]=255;
    } else {
        double remaining=1-values[3]/255;
        for(int k=0;k<3;k++) {
            double v=values[k]+background[at*4+k]*remaining;
            out[i+k]=(unsigned char)((v>255?255:v)+.5);
        }
    }
    out[i+3]=255;
}

#include "gpu_patches.cuh"
