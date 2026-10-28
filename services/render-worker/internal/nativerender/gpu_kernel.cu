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
DEVICE Color view_color(GPUParams p,const unsigned char *views,Color c,double u,double v){
    for(int i=0;i<p.view_dims[0];i++){double weight=p.view_weights[i];if(weight==0)continue;
        Color a=sample(views+i*p.view_dims[1]*p.view_dims[2]*4,p.view_dims[1],p.view_dims[2],u*p.sw-.5,v*p.sh-.5);
        for(int k=0;k<4;k++)c.v[k]=a.v[k]*weight+c.v[k]*(1-a.v[3]*weight);
    }return c;
}
DEVICE double laughter_amount(GPUParams p,double x,double y){
 double q=p.laughter_values[0];if(p.laughter_values[2]){double outside=0;double a=p.laughter_dims[4]-x,b=x-p.laughter_dims[4]-p.laughter_dims[6],c=p.laughter_dims[5]-y,d=y-p.laughter_dims[5]-p.laughter_dims[7];if(a>outside)outside=a;if(b>outside)outside=b;if(c>outside)outside=c;if(d>outside)outside=d;double width=p.laughter_dims[6]*p.laughter_values[4];double t=outside/width;t=t<0?0:(t>1?1:t);q*=t*t*(3-2*t);}return q;
}
DEVICE void laughter_delta(GPUParams p,const unsigned char*field,double x,double y,double *dx,double *dy){
 *dx=*dy=0;if(!field||x<p.laughter_dims[0]||y<p.laughter_dims[1]||x>p.laughter_dims[0]+p.laughter_dims[2]-1||y>p.laughter_dims[1]+p.laughter_dims[3]-1)return;
 Color c=sample(field,p.laughter_dims[2],p.laughter_dims[3]*2,x-p.laughter_dims[0],y-p.laughter_dims[1]);*dx=(c.v[0]*255+c.v[1]*255*256-32768)*p.laughter_values[1];*dy=(c.v[2]*255+c.v[3]*255*256-32768)*p.laughter_values[1];
}
DEVICE void baseline_delta(GPUParams p,const unsigned char*field,const unsigned char*emotion,double x,double y,double*dx,double*dy){
 *dx=*dy=0;
 if(field&&p.fw>0&&p.fh>0){double u=(x-p.ex+.5)/p.ew,v=(y-p.ey+.5)/p.eh;if(u>=0&&u<=1&&v>=0&&v<=1){Color c=sample(field,p.fw,p.fh,u*p.fw-.5,v*p.fh-.5);*dx=(c.v[0]*255-128)*p.smile*p.units;*dy=((c.v[1]*255-128)*p.smile+(c.v[2]*255-128)*p.brows)*p.units;}}
 if(p.emotion_dims[7]&&emotion){double u=(x-p.emotion_dims[0]+.5)/p.emotion_dims[2],v=(y-p.emotion_dims[1]+.5)/p.emotion_dims[3];if(u>=0&&u<=1&&v>=0&&v<=1){double ex=u*p.emotion_dims[4]-.5,ey=v*p.emotion_dims[5]-.5;ex=ex<0?0:(ex>p.emotion_dims[4]-1?p.emotion_dims[4]-1:ex);ey=ey<0?0:(ey>p.emotion_dims[5]-1?p.emotion_dims[5]-1:ey);for(int k=1;k<10;k++)if(p.emotion_weights[k]){Color c=sample(emotion,p.emotion_dims[4],p.emotion_dims[5]*10,ex,ey+k*p.emotion_dims[5]);*dx+=(c.v[0]*255-128)*p.emotion_units*p.emotion_weights[k];*dy+=(c.v[1]*255-128)*p.emotion_units*p.emotion_weights[k];}}}
}
DEVICE void combined_delta(GPUParams p,const unsigned char*field,const unsigned char*emotion,const unsigned char*laughter,double x,double y,double*dx,double*dy){double bx,by,lx,ly;double jx=0,jy=0;GPUParams base=p;
if(p.laughter_values[5]){base.smile=0;if(x>=p.laughter_dims[0]&&y>=p.laughter_dims[1]&&x<=p.laughter_dims[0]+p.laughter_dims[2]-1&&y<=p.laughter_dims[1]+p.laughter_dims[3]-1){Color j=sample(laughter,p.laughter_dims[2],p.laughter_dims[3]*2,x-p.laughter_dims[0],y-p.laughter_dims[1]+p.laughter_dims[3]);jx=(j.v[0]*255+j.v[1]*255*256-32768)*p.laughter_values[1]*p.smile;jy=(j.v[2]*255+j.v[3]*255*256-32768)*p.laughter_values[1]*p.smile;double outside=0,a=p.laughter_dims[4]-x,b=x-p.laughter_dims[4]-p.laughter_dims[6],c=p.laughter_dims[5]-y,d=y-p.laughter_dims[5]-p.laughter_dims[7];if(a>outside)outside=a;if(b>outside)outside=b;if(c>outside)outside=c;if(d>outside)outside=d;double t=outside/(p.laughter_dims[6]*p.laughter_values[4]);t=t<0?0:(t>1?1:t);double weight=t*t*(3-2*t);jx*=weight;jy*=weight;}}
baseline_delta(base,field,emotion,x+jx,y+jy,&bx,&by);bx+=jx;by+=jy;laughter_delta(p,laughter,x,y,&lx,&ly);double q=laughter_amount(p,x,y);*dx=(1-q)*bx+q*lx;*dy=(1-q)*by+q*ly;}
DEVICE Color fragment_laughter(GPUParams p,const unsigned char*subject,const unsigned char*field,const unsigned char*material,const unsigned char*emotion,const unsigned char*views,const unsigned char*laughter,double u,double v){
 double tx=u*p.sw-.5,ty=v*p.sh-.5,x=tx,y=ty;
 if((!p.laughter_values[2]&&p.laughter_values[0]>=1)||(!p.smile&&!p.brows&&!p.emotion_dims[7]))return view_color(p,views,sample(subject,p.sw,p.sh,tx,ty),u,v);
 for(int i=0;i<(int)p.laughter_values[3];i++){double dx,dy;combined_delta(p,field,emotion,laughter,x,y,&dx,&dy);double rx=x+dx-tx,ry=y+dy-ty;if(rx*rx+ry*ry<.000001)break;double xp,xyp,xm,xym,yp,yyp,ym,yym;combined_delta(p,field,emotion,laughter,x+.5,y,&xp,&xyp);combined_delta(p,field,emotion,laughter,x-.5,y,&xm,&xym);combined_delta(p,field,emotion,laughter,x,y+.5,&yp,&yyp);combined_delta(p,field,emotion,laughter,x,y-.5,&ym,&yym);double a=1+xp-xm,b=yp-ym,c=xyp-xym,d=1+yyp-yym,det=a*d-b*c;if(det<=.2)break;double sx=(d*rx-b*ry)/det,sy=(-c*rx+a*ry)/det,scale=1,norm=__builtin_sqrt(sx*sx+sy*sy);if(norm>32)scale=norm/32;x-=sx/scale;y-=sy/scale;}

 double lx,ly;laughter_delta(p,laughter,x,y,&lx,&ly);double q=laughter_amount(p,x,y);u=(x+q*lx+.5)/p.sw;v=(y+q*ly+.5)/p.sh;Color color=sample(subject,p.sw,p.sh,u*p.sw-.5,v*p.sh-.5);double smile=p.smile*(1-q);
 if(!p.laughter_values[5]&&smile>0&&material){double px=u*p.sw-p.ex-.5,py=v*p.sh-p.ey-.5;if(px>=0&&py>=0&&px<p.ew&&py<p.eh){Color ref=sample(material,p.mw,p.mh,px,py);for(int k=0;k<3;k++)color.v[k]=color.v[k]*(1-ref.v[3]*smile)+ref.v[k]*color.v[3]*smile;}}
 return view_color(p,views,color,u,v);
}
DEVICE Color fragment(GPUParams p,const unsigned char *subject,const unsigned char *field,const unsigned char *material,const unsigned char *emotion,const unsigned char *views,const unsigned char *laughter,double u,double v) {
    if(p.laughter_values[0]>0&&laughter)return fragment_laughter(p,subject,field,material,emotion,views,laughter,u,v);
    bool emotional=p.emotion_dims[7]!=0;
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
    return view_color(p,views,c,u,v);
}
// FP32 body path: keep texture interpolation and expression inversion in the
// same precision as its geometry. The FP64 path remains the pixel reference.
struct ColorF { float v[4]; };
DEVICE ColorF sample_f(const unsigned char *p,int w,int h,float x,float y) {
    x=x<0?0:(x>w-1?w-1:x); y=y<0?0:(y>h-1?h-1:y);
    int x0=(int)x,y0=(int)y,x1=x0+1<w?x0+1:w-1,y1=y0+1<h?y0+1:h-1;
    float fx=x-x0,fy=y-y0;
    int a=(y0*w+x0)*4,b=(y0*w+x1)*4,c=(y1*w+x0)*4,d=(y1*w+x1)*4;
    ColorF out;
    for(int k=0;k<4;k++) out.v[k]=((p[a+k]+(p[b+k]-p[a+k])*fx)+
        ((p[c+k]+(p[d+k]-p[c+k])*fx)-(p[a+k]+(p[b+k]-p[a+k])*fx))*fy)/255.f;
    return out;
}
DEVICE ColorF fragment_f(GPUParams p,const unsigned char *subject,const unsigned char *field,const unsigned char *material,const unsigned char *emotion,const unsigned char *views,const unsigned char *laughter,float u,float v) {
    if(p.laughter_values[0]>0&&laughter){Color d=fragment_laughter(p,subject,field,material,emotion,views,laughter,u,v);ColorF f;for(int k=0;k<4;k++)f.v[k]=(float)d.v[k];return f;}
    const float smile=(float)p.smile,brows=(float)p.brows,units=(float)p.units,emotion_units=(float)p.emotion_units;
    bool emotional=p.emotion_dims[7]!=0;
    if(smile>0 || brows>0 || emotional) {
        float tx=u*p.sw-.5f,ty=v*p.sh-.5f;
        if((tx>=p.ex-.5f && tx<=p.ex+p.ew-.5f && ty>=p.ey-.5f && ty<=p.ey+p.eh-.5f) ||
           (emotional && tx>=p.emotion_dims[0]-.5f && tx<=p.emotion_dims[0]+p.emotion_dims[2]-.5f &&
            ty>=p.emotion_dims[1]-.5f && ty<=p.emotion_dims[1]+p.emotion_dims[3]-.5f)) {
            float x=tx,y=ty;
            for(int i=0;i<(emotional?p.emotion_dims[6]:5);i++) {
                float dx=0,dy=0;
                if(field && p.fw>0 && p.fh>0) {
                    float fu=(x-p.ex+.5f)/p.ew,fv=(y-p.ey+.5f)/p.eh;
                    if(fu>=0 && fu<=1 && fv>=0 && fv<=1) {
                        ColorF c=sample_f(field,p.fw,p.fh,fu*p.fw-.5f,fv*p.fh-.5f);
                        dx=(c.v[0]*255.f-128.f)*smile*units;
                        dy=((c.v[1]*255.f-128.f)*smile+(c.v[2]*255.f-128.f)*brows)*units;
                    }
                }
                if(emotional && emotion && p.emotion_dims[4]>0 && p.emotion_dims[5]>0) {
                    float eu=(x-p.emotion_dims[0]+.5f)/p.emotion_dims[2],ev=(y-p.emotion_dims[1]+.5f)/p.emotion_dims[3];
                    if(eu>=0 && eu<=1 && ev>=0 && ev<=1) {
                        float ex=eu*p.emotion_dims[4]-.5f,ey=ev*p.emotion_dims[5]-.5f;
                        ex=ex<0?0:(ex>p.emotion_dims[4]-1?p.emotion_dims[4]-1:ex);
                        ey=ey<0?0:(ey>p.emotion_dims[5]-1?p.emotion_dims[5]-1:ey);
                        for(int k=1;k<10;k++) if(p.emotion_weights[k]!=0) {
                            ColorF ec=sample_f(emotion,p.emotion_dims[4],p.emotion_dims[5]*10,ex,ey+k*p.emotion_dims[5]);
                            float weight=(float)p.emotion_weights[k];
                            dx+=(ec.v[0]*255.f-128.f)*emotion_units*weight;
                            dy+=(ec.v[1]*255.f-128.f)*emotion_units*weight;
                        }
                    }
                }
                float nx=tx-dx,ny=ty-dy;
                if(nx==x && ny==y) break;
                x=nx; y=ny;
            }
            u=(x+.5f)/p.sw; v=(y+.5f)/p.sh;
        }
    }
    ColorF c=sample_f(subject,p.sw,p.sh,u*p.sw-.5f,v*p.sh-.5f);
    if(smile>0 && material && p.mw>0 && p.mh>0) {
        float x=(u*p.sw-p.ex)/p.ew,y=(v*p.sh-p.ey)/p.eh;
        if(x>=0 && x<=1 && y>=0 && y<=1) {
            ColorF ref=sample_f(material,p.mw,p.mh,x*p.ew-.5f,y*p.eh-.5f);
            for(int k=0;k<3;k++) c.v[k]=c.v[k]*(1.f-ref.v[3]*smile)+ref.v[k]*c.v[3]*smile;
        }
    }
    if(p.view_dims[0]){Color d;for(int k=0;k<4;k++)d.v[k]=c.v[k];d=view_color(p,views,d,u,v);for(int k=0;k<4;k++)c.v[k]=(float)d.v[k];}
    return c;
}
DEVICE double edge_gpu(GPUVertex a,GPUVertex b,double x,double y) { return (x-a.x)*(b.y-a.y)-(y-a.y)*(b.x-a.x); }

KERNEL void syntelle_body(GPUParams p,const unsigned char *subject,const unsigned char *field,const unsigned char *material,const unsigned char *emotion,const unsigned char *views,const unsigned char *laughter,
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
        Color c=fragment(p,subject,field,material,emotion,views,laughter,u,v);
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

// FP32 geometry and texture sampling; the separate FP64 kernel remains available.
DEVICE float edge_gpu_f(GPUVertex a,GPUVertex b,float x,float y) {
    return (x-(float)a.x)*((float)b.y-(float)a.y)-(y-(float)a.y)*((float)b.x-(float)a.x);
}
KERNEL void syntelle_body_fp32(GPUParams p,const unsigned char *subject,const unsigned char *field,const unsigned char *material,const unsigned char *emotion,const unsigned char *views,const unsigned char *laughter,
                    const GPUTriangle *triangles,const int *offsets,const int *ids,unsigned char *body) {
    unsigned int at=index_1d(); if(at>=(unsigned int)(p.bw*p.bh)) return;
    int x=at%p.bw,y=at/p.bw,tile=(y/16)*p.tiles_x+x/16;
    float px=x+.5f,py=y+.5f,depth=1e30f;
    unsigned char previous[4]={0,0,0,0};
    for(int j=offsets[tile];j<offsets[tile+1];j++) {
        GPUTriangle t=triangles[ids[j]];
        float e0=edge_gpu_f(t.b,t.c,px,py),e1=edge_gpu_f(t.c,t.a,px,py),e2=edge_gpu_f(t.a,t.b,px,py);
        if(e0<0 || e1<0 || e2<0 || (e0==0 && !t.flags[0]) || (e1==0 && !t.flags[1]) || (e2==0 && !t.flags[2])) continue;
        float wa=e0*(float)t.inv,wb=e1*(float)t.inv,wc=e2*(float)t.inv;
        float z=wa*(float)t.a.z+wb*(float)t.b.z+wc*(float)t.c.z;
        if(z>depth+1e-7f) continue;
        float u,v;
        if(t.flags[3]) { u=px/(float)p.bw; v=py/(float)p.bh; }
        else {
            float inverse=1.f/(wa*(float)t.a.q+wb*(float)t.b.q+wc*(float)t.c.q);
            u=(wa*(float)t.a.u+wb*(float)t.b.u+wc*(float)t.c.u)*inverse;
            v=(wa*(float)t.a.v+wb*(float)t.b.v+wc*(float)t.c.v)*inverse;
        }
        ColorF c=fragment_f(p,subject,field,material,emotion,views,laughter,u,v);
        if(t.flags[4]) {
            if(c.v[3]>=.995f || c.v[3]<=.001f) continue;
            for(int k=0;k<4;k++) c.v[k]+=previous[k]/255.f*(1.f-c.v[3]);
        } else {
            if(c.v[3]<.995f) continue;
            depth=z;
        }
        for(int k=0;k<4;k++) previous[k]=byte_color(c.v[k]);
    }
    for(int k=0;k<4;k++) body[at*4+k]=previous[k];
}

KERNEL void syntelle_compose(GPUParams p,const unsigned char *body,const unsigned char *background,
                       const GPUAxis *xs,const GPUAxis *ys,unsigned char *out) {
    int packed=p.outw==1440||p.outw==1080;
    int half=packed?p.outw/2:p.outw;
    unsigned int at=index_1d(); if(at>=(unsigned int)(half*p.outh)) return;
    int x=at%half,y=at/half,i=(y*p.outw+x)*4;
    GPUAxis xa=xs[x],ya=ys[y]; double values[4]={0,0,0,0};
    if(xa.valid && ya.valid) {
        int a=ya.a+xa.a,b=ya.a+xa.b,c=ya.b+xa.a,d=ya.b+xa.b;
        if(body[a+3]|body[b+3]|body[c+3]|body[d+3]) {
            for(int k=0;k<(p.mask_mode==1?3:4);k++) values[k]=mixd(mixd(body[a+k],body[b+k],xa.f),mixd(body[c+k],body[d+k],xa.f),ya.f);
        }
    }
    if(packed) {
        for(int k=0;k<3;k++) out[i+k]=(unsigned char)(values[k]+.5);
        unsigned char alpha=p.mask_mode==1?background[at*4]:(unsigned char)(values[3]+.5);
        for(int k=0;k<3;k++) out[i+half*4+k]=alpha;
        out[i+half*4+3]=255;
    } else if(p.mask_mode==2) {
        for(int k=0;k<3;k++) out[i+k]=(unsigned char)(values[k]+.5);
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
