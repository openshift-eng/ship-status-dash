In order to build and deploy the dashboard you must sequentially:
1. run "podman build . --platform=linux/amd64 -f images/dashboard/Dockerfile -t quay.io/sgoeddel/dashboard:latest"
2. run "podman push quay.io/sgoeddel/dashboard:latest"
3. set oc's active context to app.ci
4. run "oc rollout restart  deployment/dashboard -n ship-status"