# Scale-to-zero deployment for Syntelle

Updated October 16, 2026. The owner confirmed Azure Container Apps for two backend services with scale-to-zero: the Go API and independent Python/FastAPI processor. This replaces the earlier Google Cloud Run choice while keeping Vue separately on Cloudflare. Priorities are low cost and simple maintenance. The owner already has Azure Container Apps Express experience; evaluate that mode before fixing it for both services. No infrastructure has been provisioned.

## Selected renderer host

October 22, 2026: the owner selected **[Modal](https://modal.com/)** for the separate native Go/CUDA renderer. This replaces the earlier requirement to host rendering on CPU-only Azure Container Apps Express. The Go API and Python/FastAPI preparation service remain on Express; Vue remains on Cloudflare. The installed local renderer, character resources and provider budgets stay unchanged.

Use zero warm containers (`min_containers=0`, no idle buffer) and an explicit bounded `max_containers`; choose the GPU and capacity only after a bounded compatibility check. Modal scales Functions to zero when there are no inputs, subject to the scale-down window. An active render session must keep its invocation alive; an indefinitely running worker/control connection would prevent idle shutdown. The API must authorize admission before invoking GPU work, and termination/cancellation must release the session and invocation. [Modal scaling](https://modal.com/docs/guide/scale), [serverless WebRTC lifecycle](https://modal.com/docs/examples/webrtc_yolo).

Preserve the native Go renderer and its H.264/Opus/WebRTC contract. A small Modal deployment/entry wrapper can launch the compiled binary; do not rewrite rendering in Python. Modal supports compiled CUDA programs, but that does not verify the Syntelle image, video-capable GPU selection or NVENC driver/library exposure. Validate the existing native CUDA/NVENC pipeline explicitly. [CUDA on Modal](https://modal.com/docs/guide/cuda).

Modal documents WebRTC with ICE/STUN and TURN when needed. Its public tunnels expose TCP, which is not public UDP port mapping. Validate actual browser-to-worker connectivity and select authenticated TURN/relay capacity for restrictive networks; no relay provider is selected here. Audio/video continue between the worker and browser through the selected media route, while Go owns signaling, authorization and conversation/provider orchestration. [Modal WebRTC example](https://modal.com/docs/examples/webrtc_yolo), [Modal tunnels](https://modal.com/docs/guide/tunnels).

The current local API accepts one registered worker and the launcher requires a loopback API. Authenticated remote control, invocation dispatch, session routing and multi-worker admission therefore need explicit implementation before cloud deployment. Verify cold start, cancellation, idle shutdown and observed zero-container state with cached speech before making production cost/capacity claims. No GPU invocation or paid trial is performed or authorized by this hosting decision.

For the deployment workflow, use Modal's application definition and CLI/SDK in CI; Terraform remains appropriate for the selected Azure/Cloudflare resources. This records the intended split without adding or applying infrastructure. [Modal continuous deployment](https://modal.com/docs/guide/continuous-deployment).

## Selected cloud database

October 22, 2026: the owner selected **Azure Database for PostgreSQL** for cloud persistence, with Flexible Server as the deployment target. Local PostgreSQL remains in place for development and evidence. Region, engine version, size, networking and backup policy are still deployment details; no cloud resource, migration or credential change is performed by this decision.

Budget the database separately from application/GPU scale-to-zero. Flexible Server stop/start is a management operation; storage is still billed while compute is stopped, and a stopped server automatically starts after seven days. Do not describe the selected database as automatically pausing with application traffic. [Microsoft stop/start documentation](https://learn.microsoft.com/en-us/azure/postgresql/configure-maintain/how-to-stop-server), [database pricing](https://azure.microsoft.com/en-us/pricing/details/postgresql/flexible-server/).

Terraform can manage the server and its databases through AzureRM's `azurerm_postgresql_flexible_server` and `azurerm_postgresql_flexible_server_database` resources. Infrastructure configuration is separate from application schema migrations and importing existing evidence. No Terraform configuration or apply is included here. [Microsoft Terraform deployment guide](https://learn.microsoft.com/en-us/azure/developer/terraform/deploy-postgresql-flexible-server-database).

## Selection criteria

Infrastructure must stop its compute between demo uses. Estimate active time, allocated resources, and trial count, adding supporting services. A range based on memory remaining resident for a full month does not represent this architecture.

The main comparison was Google Cloud Run versus Azure Container Apps Express. Azure Container Apps is the current choice after selecting Azure Speech and reviewing costs. Keep Cloud Run as a historical comparison. Both offer scale-to-zero and usage-based billing. [Cloud Run overview](https://docs.cloud.google.com/run/docs/overview/what-is-cloud-run), [Express FAQ](https://learn.microsoft.com/en-us/azure/container-apps/express-faq).

## Reassessment after selecting Azure Speech

Comparison verified October 16, 2026. Use USD public rates without commitments and reference regions East US for Azure and Iowa/us-central1 for Google. Express uses Consumption prices and the same free grant, with no base environment provisioning fee. [Express billing](https://learn.microsoft.com/en-us/azure/container-apps/express-faq#pricing-and-billing).

| Mode | vCPU-second | GiB-second | 1 billable hour at 1 vCPU and 2 GiB, before free grants |
| --- | ---: | ---: | ---: |
| Azure Container Apps Consumption / Express, active usage | US$0.000024 | US$0.000003 | US$0.1080 |
| Cloud Run request-based, active usage | US$0.000024 | US$0.0000025 | US$0.1044 |
| Cloud Run instance-based, entire instance lifetime | US$0.000018 | US$0.000002 | US$0.0792 |

Sources: [Azure Container Apps East US public pricing API](https://prices.azure.com/api/retail/prices?$filter=productName%20eq%20%27Azure%20Container%20Apps%27%20and%20armRegionName%20eq%20%27eastus%27%20and%20priceType%20eq%20%27Consumption%27), [Cloud Run pricing](https://cloud.google.com/run/pricing). The Azure calculation uses Standard vCPU Active Usage and Standard Memory Active Usage meters, not Dedicated or optional environment feature charges.

As an arithmetic example, 100 billable hours with this allocation cost US$10.80 on Azure and US$10.44 on request-based Cloud Run before free grants. With compute grants fully available, these become US$5.40 and US$5.22. Both modes have a monthly reference grant of 180,000 vCPU-seconds and 360,000 GiB-seconds plus two million requests, shared per Azure subscription or Google billing account. Instance-based Cloud Run has different rates and grants. These calculations are not a total MVP estimate and do not imply 100 application-active hours equal 100 billable hours in every mode.

Container Apps replicas above the configured minimum incur active rates while running, including the wait before scaling to zero. With request-based Cloud Run, idle instances outside configured minimums do not incur that charge; instance-based billing covers their entire lifetime with a one-minute minimum. Open WebSockets keep conversation compute billable. The final budget must measure startup, scale-down waiting, concurrency, and active connections, including both Go and FastAPI workloads. [Container Apps billing](https://learn.microsoft.com/en-us/azure/container-apps/billing), [Cloud Run billable time](https://cloud.google.com/run/pricing#billable-instance-time), [Cloud Run WebSockets](https://docs.cloud.google.com/run/docs/triggering/websockets).

The owner accepted Azure because of prior Express experience and grouping the backend and Speech on one platform. Hosting Go on Azure neither removes Speech synthesis charges nor guarantees lower latency; measure region choice and the full journey. Gemini images and other conversation providers still involve external APIs. Cloudflare remains selected for the frontend. This is a planning decision; no services have been provisioned.

Keep Go and FastAPI separate with minimum zero; replace earlier supporting authentication, queue, and persistence proposals with Azure-compatible choices. Before fixing Express for both, verify WebSockets, request duration, and processor memory. Express does not provide all standard Container Apps features, including Jobs, GPU, or service discovery. [Express feature matrix](https://learn.microsoft.com/en-us/azure/container-apps/express-overview).

## Evaluated platforms

| Option | Fit | Pending verification |
| --- | --- | --- |
| Azure Container Apps Express | Candidate mode within the selected platform, based on previous experience, simplified deployment, and automatic scale-to-zero. | Test voice transport and image preparation within Express limits. |
| Standard Azure Container Apps Consumption | Alternative mode of the selected platform if controls or features unavailable in Express are required. | Define ingress, authentication, scaling, and durable processor dispatch. Jobs has not been selected. |
| Google Cloud Run | Compared alternative, replaced by Azure in the current decision. | Retain cost references; no deployment planned. |

Express provides temporary storage but not Container Apps Jobs or Azure Files. General Container Apps documentation supports WebSockets; verify the actual Express voice journey before fixing the mode. [Express features](https://learn.microsoft.com/en-us/azure/container-apps/express-overview), [Container Apps ingress](https://learn.microsoft.com/en-us/azure/container-apps/ingress-overview).

Cloud Run supports WebSockets and bills the instance while a connection remains open. End connections with the conversation to permit scale-to-zero. [Cloud Run documentation](https://docs.cloud.google.com/run/docs/triggering/websockets).

## Historical comparison with the Google equivalent

Cloud Run is the closest functional Container Apps equivalent for this application: managed containers, source deployment for supported languages, HTTPS endpoints, and scale-to-zero without traffic. The previous proposal used HTTP services for the Go API and Python/FastAPI processor. [Official overview](https://docs.cloud.google.com/run/docs/overview/what-is-cloud-run).

The superseded proposal had two Cloud Run services with minimum zero and IAM authentication. Supporting candidates were Cloud Tasks for durable dispatch, Firestore for quotas and progress, and Cloud Storage for temporary files. None was contracted and they are outside the current Azure architecture. Keep them only as comparison history. Final character storage remains in the browser. [Service authentication](https://docs.cloud.google.com/run/docs/authenticating/service-to-service), [Firestore pricing](https://cloud.google.com/firestore/pricing), [Cloud Storage pricing](https://cloud.google.com/storage/pricing).

Source deployment can build containers automatically. Cloud Build and Artifact Registry have their own billing terms and are not included in Cloud Run compute. [Source deployment](https://docs.cloud.google.com/run/docs/deploying-source-code), [Cloud Run pricing](https://cloud.google.com/run/pricing).

## Frontend on Cloudflare

Workers Static Assets on the free plan is recommended for the compiled Vue SPA. Cloudflare recommends Workers for new projects and documents free, unlimited static requests with no additional asset storage charge, subject to platform limits. The frontend does not require a dynamic Worker or API proxy. Pages was also evaluated and offers free static assets, but is not the recommendation for a new project here. [Official recommendation](https://developers.cloudflare.com/pages/), [Static Assets pricing and limits](https://developers.cloudflare.com/workers/static-assets/billing-and-limitations/).

Vue and Go deploy independently. The browser connects directly to Go through HTTPS/WSS; origins and sessions remain to be finalized. The static interface loads without waiting for Go startup, while backend operations retain their latency. Static frontend budgeting can start at US$0 under free-plan conditions; any custom domain, backend, visitor storage, and APIs are separate.

## Google active compute reference

The examples compare rates under an assumed active resource allocation. They are not a budget for the selected two-service architecture, which must add measured Go and FastAPI resource time and supporting services.

For the same active time used below, request-based service rates in Iowa are US$0.000024 per vCPU-second and US$0.0000025 per GiB-second. At 100 eight-minute trials with 1 vCPU and 2 GiB, active compute totals US$1.392 before grants; 1,000 trials total US$13.92. These are active resource calculations, not full WebSocket session or Jobs estimates, which depend on billing mode and actual billable duration.

This mode lists 180,000 vCPU-seconds, 360,000 GiB-seconds, and two million free monthly requests shared per billing account. The 100-trial example fits those grants if available. Registry, builds, data, transfer, and APIs remain separate. [Official rates and grants](https://cloud.google.com/run/pricing).

## Azure active compute calculation

Compare East US Standard Consumption rates checked through the [public pricing API](https://prices.azure.com/api/retail/prices?$filter=productName%20eq%20%27Azure%20Container%20Apps%27%20and%20armRegionName%20eq%20%27eastus%27%20and%20skuName%20eq%20%27Standard%27): US$0.000024 per vCPU-second and US$0.000003 per active memory GiB-second.

Example assumption: each trial prepares a character for two minutes and maintains conversation for six minutes, with 1 vCPU and 2 GiB allocated throughout. This is eight active compute minutes per trial. Actual duration and required resources have not been measured. Sum trials without deducting possible concurrency overlap.

| Monthly trials | vCPU-seconds | GiB-seconds | Compute before free grants | Compute with the full monthly grant available |
| --- | ---: | ---: | ---: | ---: |
| 10 | 4,800 | 9,600 | US$0.144 | US$0 |
| 100 | 48,000 | 96,000 | US$1.44 | US$0 |
| 1,000 | 480,000 | 960,000 | US$14.40 | US$9.00 |

The Consumption grant is 180,000 vCPU-seconds, 360,000 GiB-seconds, and two million monthly requests per subscription, shared with other workloads. Do not attribute the full amount to Syntelle if other projects already consume it. [Container Apps billing](https://learn.microsoft.com/en-us/azure/container-apps/billing).

The table covers active compute only under these assumptions. Exclusions include startup, time before scale-to-zero, requests beyond free grants, transfer, image registry, storage, and logs. Gemini, recognition, and TTS are also separate in [providers and costs](research.md). A zero in one column does not mean the entire application is free.

## Required architecture changes

- Keep minimum compute at zero and verify that the application reaches zero without traffic.
- Close speech and recognition connections when finished or their quotas are exhausted.
- Use external durable storage for quotas, administrative settings, reservations, and jobs; temporary disk is not authoritative. Operational persistence does not require visitor accounts; the private panel has separate protected access.
- Run preparation during an active request or a durably triggered job with persistent state. Do not rely on background work started after responding to the browser.
- Load the depth model only when needed and measure startup, memory, and total duration. Fast platform startup does not guarantee fast model startup.
- Select registry, logging, and storage without unnecessary always-on services and include their prices explicitly.

## Confirmed decision and next steps

Vue has a separate static deployment on Cloudflare, with Workers Static Assets recommended. The Go API and Python/FastAPI processor are separate Azure Container Apps services with minimum zero and independent sizing. Azure Speech is selected as managed TTS and visemes called from Go; no voice server is self-hosted. Select the Container Apps mode, service authentication, durable dispatch, operational persistence, temporary storage, and cross-origin session strategy compatible with Azure. Earlier Google supporting service proposals are inactive. Optimize and measure the processor before fixing resources or adding GPU.

The owner reports Container Apps Express cold starts under two seconds. Use this as an evaluation reference, not measured Syntelle performance. Evaluate API startup from zero separately from model loading and first spoken response. Keep image processing outside API startup.
