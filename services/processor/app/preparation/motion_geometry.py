"""Validate the browser's bounded 2.5D deformation before packaging assets."""
from itertools import product
import numpy as np
from PIL import Image
from .faces import PreparationError


def smooth(low,high,value):
    p=np.clip((value-low)/(high-low),0,1)
    return p*p*(3-2*p)


def deform(points,depth,anatomy,size,angles,breath):
    """Same units and order as characterRenderer's vertex program."""
    pivot=np.array([anatomy['head_pivot'][0],anatomy['chin_y']])
    height=anatomy['face_height'];width=anatomy['face_width']
    weight=1-smooth(pivot[1],pivot[1]+height*.55,points[:,1])
    p=np.column_stack((points-pivot,(depth-.5)*width*.35))
    yaw,pitch,roll=np.radians(angles);cy,sy=np.cos(yaw),np.sin(yaw);cp,sp=np.cos(pitch),np.sin(pitch);cr,sr=np.cos(roll),np.sin(roll)
    a=np.column_stack((cy*p[:,0]+sy*p[:,2],p[:,1],-sy*p[:,0]+cy*p[:,2]))
    b=np.column_stack((a[:,0],cp*a[:,1]-sp*a[:,2],sp*a[:,1]+cp*a[:,2]))
    rotated=np.column_stack((cr*b[:,0]-sr*b[:,1],sr*b[:,0]+cr*b[:,1]))
    result=points+weight[:,None]*(rotated-p[:,:2])
    torso=smooth(pivot[1]+height*.2,pivot[1]+height*.65,result[:,1])*(1-smooth(size[1]-height*.15,size[1],result[:,1]))
    result[:,1]+=torso*breath
    result[:,0]+=torso*(result[:,0]-pivot[0])*breath/max(height,1)*.25
    return result


def validate(depth,anatomy,size,settings):
    n=64;yy,xx=np.mgrid[0:n+1,0:n+1];points=np.column_stack((xx.ravel()/n*size[0],yy.ravel()/n*size[1]))
    samples=np.asarray(depth.convert('L').resize((n+1,n+1),Image.Resampling.BILINEAR)).ravel()/255
    grid=np.arange((n+1)**2).reshape(n+1,n+1);a=grid[:-1,:-1].ravel();b=a+1;c=a+n+1;d=c+1
    triangles=np.vstack((np.column_stack((a,b,c)),np.column_stack((c,b,d))))
    minimum=float('inf');checks=0
    for yaw,pitch,roll,breath in product((-1,0,1),repeat=4):
        moved=deform(points,samples,anatomy,size,[yaw*settings['yaw_degrees'],pitch*settings['pitch_degrees'],roll*settings['roll_degrees']],breath*anatomy['face_height']*settings['breath_face_height_fraction'])
        tri=moved[triangles];ab=tri[:,1]-tri[:,0];ac=tri[:,2]-tri[:,0]
        area=ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0]
        minimum=min(minimum,float(area.min()));checks+=len(triangles)
    if not np.isfinite(minimum) or minimum<=0:raise PreparationError('Motion bounds invert the character mesh; package rejected.')
    return {'version':'motion-grid-validation/1','grid':n,'bounded_configurations':81,'triangles_checked':checks,'minimum_signed_double_area':minimum}
