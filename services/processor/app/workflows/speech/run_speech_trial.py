"""Start the local Go SDK container using server secrets, without recording them."""
import os
import subprocess
from urllib.parse import urlparse,urlunparse

from dotenv import dotenv_values

from app.workflows.visemes.viseme_trial import ROOT


def main():
    # Explicit project configuration wins over unrelated account/work environment.
    values={**os.environ,**{k:v for k,v in dotenv_values(ROOT/'.env').items() if v is not None}}
    for name in ['AZURE_SPEECH_KEY','AZURE_SPEECH_REGION','SYNTELLE_DATABASE_URL']:
        if not values.get(name):raise SystemExit(f'Configure {name} in the local .env before starting speech.')
    if values.get('SYNTELLE_SPEECH_TRIAL')!='1':raise SystemExit('Set SYNTELLE_SPEECH_TRIAL=1 in the local .env.')
    address=urlparse(values['SYNTELLE_DATABASE_URL'])
    if address.hostname in ('127.0.0.1','localhost'):
        values['SYNTELLE_DATABASE_URL']=urlunparse(address._replace(netloc=address.netloc.replace(address.hostname,'host.docker.internal')))
    values['PROCESSOR_URL']='http://host.docker.internal:8000'
    container='syntelle-speech-trial-12'
    existing=subprocess.run(['docker','container','inspect',container,'--format','{{.Name}}'],capture_output=True,text=True)
    if existing.returncode==0:
        if existing.stdout.strip()!='/'+container:raise SystemExit('Unexpected speech container identity.')
        subprocess.run(['docker','rm','-f',container],check=True,stdout=subprocess.DEVNULL)
    arguments=['docker','run','-d','--name',container,'-p','127.0.0.1:8082:8080']
    for name in ['AZURE_SPEECH_KEY','AZURE_SPEECH_REGION','SYNTELLE_DATABASE_URL','SYNTELLE_SPEECH_TRIAL','PROCESSOR_URL']:
        arguments.extend(['--env',name])
    subprocess.run([*arguments,'syntelle-api:speech-12'],env=values,check=True,stdout=subprocess.DEVNULL)
    print('Local Dragon HD API started on 127.0.0.1:8082; no synthesis request was made.')


if __name__=='__main__':main()
