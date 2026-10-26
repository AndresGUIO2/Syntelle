// Source-bound mouth, eyes, skin and exact integer BT.601 conversion.
DEVICE double pmix(double a,double b,double f){return a*(1-f)+b*f;}
DEVICE double mind(double a,double b){return a<b?a:b;}
DEVICE double maxd(double a,double b){return a>b?a:b;}
DEVICE double absd(double a){return a<0?-a:a;}
DEVICE double clipd(double a,double lo,double hi){return mind(hi,maxd(lo,a));}
DEVICE double sqrtd(double a){double r;asm("sqrt.rn.f64 %0, %1;":"=d"(r):"d"(a));return r;}
// Range-reduced double math avoids a runtime CUDA toolkit or approximate
// single-precision lighting. Arguments here are bounded nonpositive values.
DEVICE double expd(double x){
    if(x < -700)return 0;
    int n=(int)(x/0.69314718055994530942-.5);
    double r=(x-n*0.69314718055994528623)-n*2.3190468138462995584e-17;
    double sum=1,term=1;for(int i=1;i<=18;i++){term*=r/i;sum+=term;}
    union { unsigned long long bits;double value; } scale;
    scale.bits=(unsigned long long)(n+1023)<<52;return sum*scale.value;
}
DEVICE double exposure(double value){
    double z=(value-1)/(value+1),term=z,sum=z;
    for(int i=3;i<=151;i+=2){term*=z*z;sum+=term/i;}
    return expd((2*sum)*(1/2.2));
}
DEVICE double distance2(double x,double y,double ax,double ay,double bx,double by){
    double dx=bx-ax,dy=by-ay,t=clipd(((x-ax)*dx+(y-ay)*dy)/maxd(dx*dx+dy*dy,.0001),0,1);
    dx=x-(ax+dx*t);dy=y-(ay+dy*t);return dx*dx+dy*dy;
}
DEVICE bool crosses(double x,double y,double ax,double ay,double bx,double by){
    return (ay>y)!=(by>y) && x < (bx-ax)*(y-ay)/(by-ay)+ax;
}
DEVICE double smooth(double a,double b,double v){double t=clipd((v-a)/(b-a),0,1);return t*t*(3-2*t);}
DEVICE Color color_mix(Color a,Color b,double f){for(int k=0;k<4;k++)a.v[k]=pmix(a.v[k],b.v[k],f);return a;}
DEVICE const unsigned char *oral_tex(const unsigned char *atlas,GPUOralParams p,int index){return atlas+(unsigned long long)index*p.dims[0]*p.dims[1]*4;}
DEVICE Color oral_sample(const unsigned char *atlas,GPUOralParams p,int index,double x,double y){return sample(oral_tex(atlas,p,index),p.dims[0],p.dims[1],x,y);}
DEVICE double oral_edge(const double *a,const double *b,double x,double y){return (b[0]-a[0])*(y-a[1])-(b[1]-a[1])*(x-a[0]);}
DEVICE Color oral_shade(const unsigned char *atlas,GPUOralParams p,int x,int y,double *from,double *to,double alpha){
    const double *v=p.values;double amount=v[0],cavity=v[1],px=x+.5,py=y+.5;
    Color color;
    if(amount==0)color=oral_sample(atlas,p,p.dims[4],from[0]-.5,from[1]-.5);
    else if(amount==1)color=oral_sample(atlas,p,p.dims[7],to[0]-.5,to[1]-.5);
    else color=color_mix(oral_sample(atlas,p,p.dims[4],from[0]-.5,from[1]-.5),oral_sample(atlas,p,p.dims[7],to[0]-.5,to[1]-.5),amount);
    if(p.dims[15]>0 && v[14]>0){Color lip=oral_sample(atlas,p,p.dims[15],from[0]-.5,from[1]-.5);for(int k=0;k<3;k++)color.v[k]=pmix(color.v[k],lip.v[k],lip.v[3]*v[14]);}
    bool semantic=p.dims[13];const double *inner=v+16;double gap=inner[31]-inner[11];
    if(semantic || (px>=v[6]-1 && py>=v[7]-1 && px<=v[8]+1 && py<=v[9]+1 && gap>v[13])){
        double coverage=0,apertureDistance=0,openingRatio=0;
        if(semantic){
            double d;
            if(amount==0)d=(oral_sample(atlas,p,p.dims[5],from[0]-.5,from[1]-.5).v[1]*255-128)/8;
            else if(amount==1)d=(oral_sample(atlas,p,p.dims[8],to[0]-.5,to[1]-.5).v[1]*255-128)/8;
            else d=pmix((oral_sample(atlas,p,p.dims[5],from[0]-.5,from[1]-.5).v[1]*255-128)/8,(oral_sample(atlas,p,p.dims[8],to[0]-.5,to[1]-.5).v[1]*255-128)/8,amount);
            coverage=smooth(-.5,.5,d);
            apertureDistance=maxd(0,d);
            if(v[56]>0)openingRatio=pmix(oral_sample(atlas,p,p.dims[5],from[0]-.5,from[1]-.5).v[2],oral_sample(atlas,p,p.dims[8],to[0]-.5,to[1]-.5).v[2],amount);
        }else{
            double edge=distance2(px,py,inner[38],inner[39],inner[0],inner[1]);
            bool inside=crosses(px,py,inner[38],inner[39],inner[0],inner[1]);
            for(int i=0;i<19;i++){edge=mind(edge,distance2(px,py,inner[2*i],inner[2*i+1],inner[2*i+2],inner[2*i+3]));if(crosses(px,py,inner[2*i],inner[2*i+1],inner[2*i+2],inner[2*i+3]))inside=!inside;}
            double d=sqrtd(edge);if(!inside)d=-d;coverage=smooth(-.5,.65,d);
            apertureDistance=maxd(0,d);if(v[56]>0)openingRatio=maxd(0,gap)/v[56];
        }
        if(coverage>0 && (semantic || gap>v[13])){
            Color interior;
            if(cavity==0)interior=oral_sample(atlas,p,p.dims[6],from[0]-.5,from[1]-.5);
            else if(cavity==1)interior=oral_sample(atlas,p,p.dims[9],to[0]-.5,to[1]-.5);
            else interior=color_mix(oral_sample(atlas,p,p.dims[6],from[0]-.5,from[1]-.5),oral_sample(atlas,p,p.dims[9],to[0]-.5,to[1]-.5),cavity);
            Color upper=oral_sample(atlas,p,p.dims[10],x-v[2],y-v[3]),lower=oral_sample(atlas,p,p.dims[11],x-v[4],y-v[5]);
            for(int k=0;k<3;k++){upper.v[k]/=maxd(upper.v[3],.00001);lower.v[k]/=maxd(lower.v[3],.00001);}
            if(upper.v[3]>0 || lower.v[3]>0){
                double edge=mind(1e8,distance2(px,py,inner[38],inner[39],inner[0],inner[1]));
                for(int i=0;i<19;i++)edge=mind(edge,distance2(px,py,inner[2*i],inner[2*i+1],inner[2*i+2],inner[2*i+3]));
                double xd=absd(px-(inner[0]+inner[20])*.5)/maxd(1,(inner[20]-inner[0])*.5);
                double illumination=maxd(semantic?.22:.68,mind(1,1-v[12]*xd*xd-v[10]*expd(-sqrtd(edge)/maxd(.1,v[11]))));
                if(v[56]>0){double ambient=pmix(v[57],1,smooth(0,v[58],openingRatio));illumination=maxd(.12,mind(1,1-v[12]*xd*xd-v[10]*expd(-apertureDistance/v[59])))*ambient;}
                double e=exposure(illumination);for(int k=0;k<3;k++){upper.v[k]*=e;lower.v[k]*=e;}
            }
            for(int k=0;k<3;k++){double oral=pmix(interior.v[k],lower.v[k],lower.v[3]);oral=pmix(oral,upper.v[k],upper.v[3]);color.v[k]=pmix(color.v[k],oral,coverage);}
        }
    }
    color.v[3]=alpha;return color;
}
DEVICE void patch_over(unsigned char *subject,const unsigned char *original,const unsigned char *skin,int sw,int x,int y,Color c,const int *skin_rect){
    int at=(y*sw+x)*4;const unsigned char *base=original+at;
    if(skin && x>=skin_rect[0] && y>=skin_rect[1] && x<skin_rect[0]+skin_rect[2] && y<skin_rect[1]+skin_rect[3])base=skin+((y-skin_rect[1])*skin_rect[2]+x-skin_rect[0])*4;
    // Overlay quantization is part of the original CPU source-atop contract.
    unsigned char a=byte_color(c.v[3]);double alpha=a/255.0,ba=original[at+3]/255.0;
    for(int k=0;k<3;k++)subject[at+k]=byte_color(byte_color(c.v[k])/255.0*alpha*ba+base[k]/255.0*(1-alpha));
    subject[at+3]=original[at+3];
}
KERNEL void syntelle_oral(GPUOralParams p,const GPUOralTriangle *triangles,const unsigned char *atlas,unsigned char *subject,const unsigned char *original,const unsigned char *skin,int sw,GPUSkinParams sp){
    int at=index_1d(),w=p.dims[0],h=p.dims[1];if(at>=w*h)return;int x=at%w,y=at/w;double px=x+.5,py=y+.5;Color color={{0,0,0,0}};
    const unsigned char *a=oral_tex(atlas,p,p.dims[5]),*b=oral_tex(atlas,p,p.dims[8]);
    double alpha=pmix(a[at*4],b[at*4],p.values[0])/255;
    bool movingCoverage=(p.dims[12]&2)!=0;
    if(movingCoverage || alpha>=.5/255)for(int j=0;j<p.dims[14];j++){
        GPUOralTriangle t=triangles[j];double e0=oral_edge(t.p+2,t.p+4,px,py),e1=oral_edge(t.p+4,t.p,px,py),e2=oral_edge(t.p,t.p+2,px,py);
        if(e0<0 || e1<0 || e2<0 || (e0==0&&!t.flags[0]) || (e1==0&&!t.flags[1]) || (e2==0&&!t.flags[2]))continue;
        double wa=e0/t.area,wb=e1/t.area,wc=e2/t.area,from[2],to[2];
        for(int k=0;k<2;k++){from[k]=t.from[k]*wa+t.from[2+k]*wb+t.from[4+k]*wc;to[k]=t.to[k]*wa+t.to[2+k]*wb+t.to[4+k]*wc;}
        if(p.values[0]==0&&!p.dims[12]){from[0]=px;from[1]=py;}
        if(p.values[0]==1&&!p.dims[12]){to[0]=px;to[1]=py;}
        if(movingCoverage){
            alpha=pmix(oral_sample(atlas,p,p.dims[5],from[0]-.5,from[1]-.5).v[0],oral_sample(atlas,p,p.dims[8],to[0]-.5,to[1]-.5).v[0],p.values[0]);
            if(alpha<.5/255)continue;
        }
        color=oral_shade(atlas,p,x,y,from,to,alpha);
    }
    patch_over(subject,original,skin,sw,x+p.dims[2],y+p.dims[3],color,sp.dims);
}

KERNEL void syntelle_skin(GPUSkinParams p,unsigned char *subject,const unsigned char *original,unsigned char *skin,const unsigned char *frames,const unsigned char *laughs,int sw){
    int at=index_1d(),w=p.dims[2],h=p.dims[3];if(at>=w*h)return;
    int global=((at/w+p.dims[1])*sw+at%w+p.dims[0])*4;double a=original[global+3]/255.0;
    const unsigned char *lo=frames+(p.dims[4]*w*h+at)*4,*hi=frames+(p.dims[5]*w*h+at)*4;
    double weight=p.values[0],alpha=mixd(lo[3],hi[3],weight)/255;
    for(int k=0;k<3;k++){
        double value=original[global+k]/255.0;
        if(alpha>0)value=mixd(lo[k],hi[k],weight)/255*a/alpha;
        if(p.values[2]>0 && p.dims[6]){
            const unsigned char *la=laughs+(p.dims[4]*w*h+at)*4,*lb=laughs+(p.dims[5]*w*h+at)*4;
            double opacity=mixd(la[3],lb[3],weight)/255;
            if(opacity>0)value=mixd(value,mixd(la[k],lb[k],weight)/255*a/opacity,p.values[2]);
        }
        skin[at*4+k]=p.values[1]==0?original[global+k]:byte_color(value);subject[global+k]=skin[at*4+k];
    }
    skin[at*4+3]=original[global+3];subject[global+3]=original[global+3];
}

DEVICE Color iris_sample(const Color *iris,int w,int h,double x,double y){
    int ix=(int)x,iy=(int)y;if(x<ix)ix--;if(y<iy)iy--;
    double fx=x-ix,fy=y-iy;int x0=(int)clipd(ix,0,w-1),x1=(int)clipd(ix+1,0,w-1),y0=(int)clipd(iy,0,h-1),y1=(int)clipd(iy+1,0,h-1);
    return color_mix(color_mix(iris[y0*w+x0],iris[y0*w+x1],fx),color_mix(iris[y1*w+x0],iris[y1*w+x1],fx),fy);
}
DEVICE Color gazed(const unsigned char *atlas,const Color *iris,int w,int h,int x,int y,double dx,double dy,Color base){
    if(dx==0&&dy==0)return base;
    const unsigned char *ownership=atlas+4*w*h*4,*sclera=atlas+3*w*h*4;
    Color own=sample(ownership,w,h,x,y);if(own.v[0]==0)return base;
    Color shifted=sample(ownership,w,h,x-dx,y-dy),i=iris_sample(iris,w,h,x-dx,y-dy),clean=sample(sclera,w,h,x,y);
    double missing=clipd(shifted.v[1]-i.v[3],0,1);Color color;
    for(int k=0;k<3;k++)color.v[k]=pmix(base.v[k],i.v[k]+base.v[k]*missing+clean.v[k]*maxd(0,1-i.v[3]-missing),own.v[0]);
    color.v[3]=1;return color;
}
DEVICE double upper_map(double y,double edge,double source,double band){return y>=edge-band?y+source-edge:y*(source-band)/maxd(edge-band,1);}
DEVICE double lower_map(double y,double edge,double source,double band,double height){return y<=edge+band?y+source-edge:height-1+(y-height+1)*(source+band-height+1)/mind(edge+band-height+1,-1);}
KERNEL void syntelle_eyes(GPUEyeParams p,const unsigned char *atlas,const Color *iris,const double *curves,unsigned char *subject,const unsigned char *original,const unsigned char *skin,int sw,GPUSkinParams sp){
    int at=index_1d(),w=p.dims[0],h=p.dims[1];if(at>=w*h)return;int x=at%w,y=at/w;double closure=p.values[0];
    Color base=sample(atlas,w,h,x,y),target=sample(atlas+w*h*4,w,h,x,y),color=base;double alpha=atlas[2*w*h*4+at*4]/255.0;
    if(closure==1)color=color_mix(base,target,alpha);
    else {
        int eye=x+.5>=p.values[1]?1:0;Color eye_color=gazed(atlas,iris,w,h,x,y,p.values[3+eye*2],p.values[4+eye*2],base);
        if(closure==0 && p.values[7]==0)color=eye_color;
        else {
            color=color_mix(base,target,closure);const double *c=curves+4*x;
            if(c[3]>0){
                double upper=pmix(c[0],c[2],closure),lower=pmix(c[1],c[2],closure)-(c[1]-c[0])*p.values[7]*(1-closure);
                double band=mind(p.values[2],mind(mind(c[0],c[2]),mind(h-1-c[1],h-1-c[2]))*.8);
                double sy=maxd(lower_map(y,lower,c[1],band,h),c[1]+.5),cy=lower_map(y,lower,c[2],band,h);
                if(y<upper){sy=mind(upper_map(y,upper,c[0],band),c[0]-.5);cy=upper_map(y,upper,c[2],band);}
                Color skin_color=color_mix(sample(atlas,w,h,x,sy),sample(atlas+w*h*4,w,h,x,cy),closure);
                double coverage=clipd(mind(y+.5,lower)-maxd(y-.5,upper),0,1);color=color_mix(skin_color,eye_color,coverage);
            }
            color=color_mix(base,color,alpha);
        }
    }
    color.v[3]=1;patch_over(subject,original,skin,sw,x+p.dims[2],y+p.dims[3],color,sp.dims);
}

DEVICE double reference_jaw_weight(double x,double y,GPUReferenceParams p){
    const double *v=p.values;
    double left=v[60]+p.dims[10],right=v[80]+p.dims[10],width=maxd(1,right-left);
    x=(x-(left+right)*.5)/(width*.85);
    double horizontal=maxd(0,1-x*x);horizontal*=horizontal;
    return horizontal*smooth(v[71]+p.dims[11],v[91]+p.dims[11],y)*(1-smooth(v[3],v[3]+width*.3,y));
}
DEVICE Color reference_interior(const unsigned char *atlas,const Color *iris,int w,int h,double x,double y,double dx,double dy){
    Color base=sample(atlas,w,h,x,y);if(dx==0&&dy==0)return base;
    Color own=sample(atlas+4*w*h*4,w,h,x-dx,y-dy),i=iris_sample(iris,w,h,x-dx,y-dy),clean=sample(atlas+3*w*h*4,w,h,x,y);
    double missing=clipd(own.v[1]-i.v[3],0,1);
    for(int k=0;k<3;k++)base.v[k]=i.v[k]+base.v[k]*missing+clean.v[k]*maxd(0,1-i.v[3]-missing);
    return base;
}
KERNEL void syntelle_reference(GPUReferenceParams p,const unsigned char *faces,const double *profiles,const unsigned char *eyes,const Color *iris,const double *source_curves,const unsigned char *oral,unsigned char *subject,const unsigned char *original,int sw){
    int at=index_1d(),w=p.dims[2],h=p.dims[3];if(at>=w*h)return;
    int x=at%w,y=at/w,gx=x+p.dims[0],gy=y+p.dims[1],global=(gy*sw+gx)*4;
    if(p.dims[15]){for(int k=0;k<4;k++)subject[global+k]=original[global+k];return;}
    const double *v=p.values;double sy=gy;
    for(int i=0;i<3;i++)sy=gy-v[2]*reference_jaw_weight(gx,sy,p);
    Color color=color_mix(sample(faces+p.dims[4]*w*h*4,w,h,x,sy-p.dims[1]),sample(faces+p.dims[5]*w*h*4,w,h,x,sy-p.dims[1]),v[0]);
    double alpha=original[global+3]/255.0;
    if(color.v[3]>0)for(int k=0;k<3;k++)color.v[k]*=alpha/color.v[3];color.v[3]=alpha;
    int ox=gx-p.dims[10],oy=gy-p.dims[11],ow=p.dims[12],oh=p.dims[13];
    if(ox>=0&&oy>=0&&ox<ow&&oy<oh){
        double px=ox+.5,py=oy+.5,distance=1e30;bool inside=false;
        for(int i=0;i<20;i++){int j=(i+1)%20;const double *a=v+20+i*2,*b=v+20+j*2;
            if(crosses(px,py,a[0],a[1],b[0],b[1]))inside=!inside;
            distance=mind(distance,distance2(px,py,a[0],a[1],b[0],b[1]));}
        double d=sqrtd(distance);if(!inside)d=-d;double coverage=smooth(-.5,.65,d);
        if(coverage>0){
            Color cavity=sample(oral+p.dims[14]*ow*oh*4,ow,oh,ox,oy-v[2]*.5);
            Color lower=sample(oral+(p.dims[14]+2)*ow*oh*4,ow,oh,ox-v[9],oy-v[10]),upper=sample(oral+(p.dims[14]+1)*ow*oh*4,ow,oh,ox,oy);
            double light=1;
            if(v[100]>0 && v[102]>0){
                double closure=clipd(1-(v[51]-v[31])/v[102],0,1);
                light=exposure(maxd(.55,1-v[100]*closure*expd(-maxd(0,d)/v[101])));
            }
            for(int k=0;k<3;k++){lower.v[k]*=light;upper.v[k]*=light;}
            for(int k=0;k<3;k++){cavity.v[k]=lower.v[k]+cavity.v[k]*(1-lower.v[3]);cavity.v[k]=upper.v[k]+cavity.v[k]*(1-upper.v[3]);color.v[k]=pmix(color.v[k],cavity.v[k]*alpha,coverage);}
        }
    }
    int ex=gx-p.dims[6],ey=gy-p.dims[7],ew=p.dims[8],eh=p.dims[9];
    if(ex>=0&&ey>=0&&ex<ew&&ey<eh){
        const double *a=profiles+4*(p.dims[4]*ew+ex),*b=profiles+4*(p.dims[5]*ew+ex);
        if(a[3]>0 || b[3]>0){
            double upper=pmix(a[0],b[0],v[0]),lower=pmix(a[1],b[1],v[0]),seam=pmix(a[2],b[2],v[0]);
            double open=clipd(mind(ey+.5,lower)-maxd(ey-.5,upper),0,1);
            {
                double cu=pmix(upper,seam,v[4]),cl=pmix(lower,seam,v[4]);
                double visible=clipd(mind(ey+.5,cl)-maxd(ey-.5,cu),0,1);
                int eye=ex+.5>=v[5]?1:0;
                double sx=ex-v[15+eye*2],sy=ey-v[16+eye*2];
                Color interior=reference_interior(eyes,iris,ew,eh,sx,sy,v[11+eye*2],v[12+eye*2]);
                double band=mind(v[6],mind(mind(upper,seam),mind(eh-1-lower,eh-1-seam))*.8);
                double open_y=maxd(lower_map(ey,cl,lower,band,eh),lower+.5),closed_y=lower_map(ey,cl,source_curves[4*ex+2],band,eh);
                if(ey<cu){open_y=mind(upper_map(ey,cu,upper,band),upper-.5);closed_y=upper_map(ey,cu,source_curves[4*ex+2],band);}
                double fx=ex+p.dims[6]-p.dims[0],fy=open_y+p.dims[7]-p.dims[1];
                Color open_skin=color_mix(sample(faces+p.dims[4]*w*h*4,w,h,fx,fy),sample(faces+p.dims[5]*w*h*4,w,h,fx,fy),v[0]);
                Color closed=sample(eyes+ew*eh*4,ew,eh,sx,closed_y);
                Color result=color_mix(color_mix(open_skin,closed,v[4]),interior,visible);
                double coverage=maxd(open,sample(eyes+2*ew*eh*4,ew,eh,ex,ey).v[0]);
                for(int k=0;k<3;k++)color.v[k]=pmix(color.v[k],result.v[k]*alpha,coverage);
            }
        }
    }
    for(int k=0;k<4;k++)subject[global+k]=byte_color(color.v[k]);
}

KERNEL void syntelle_i420(GPUOutput p,unsigned char *rgba){
    int at=index_1d(),count=p.width*p.height;if(at>=count)return;int x=at%p.width,y=at/p.width,i=at*4;
    if(p.marker&&x<32&&y<32){rgba[i]=255;rgba[i+1]=0;rgba[i+2]=255;rgba[i+3]=255;}
    int r=rgba[i],g=rgba[i+1],b=rgba[i+2];
    unsigned char *yp=(unsigned char*)p.planes[0],*up=(unsigned char*)p.planes[1],*vp=(unsigned char*)p.planes[2];
    yp[y*p.strides[0]+x]=(66*r+129*g+25*b+0x1080)>>8;
    if((x&1)||(y&1))return;
    int j=i+p.width*4;
    int ar,ag,ab;
    if(p.marker&&x<32&&y<32){ar=255;ag=0;ab=255;}
    else {ar=(r+rgba[i+4]+rgba[j]+rgba[j+4]+2)>>2;ag=(g+rgba[i+5]+rgba[j+1]+rgba[j+5]+2)>>2;ab=(b+rgba[i+6]+rgba[j+2]+rgba[j+6]+2)>>2;}
    // Match libyuv's limited-range BT.601 integer chroma coefficients.
    up[(y/2)*p.strides[1]+x/2]=(112*ab-74*ag-38*ar+0x8000)>>8;
    vp[(y/2)*p.strides[2]+x/2]=(112*ar-94*ag-18*ab+0x8000)>>8;
}
