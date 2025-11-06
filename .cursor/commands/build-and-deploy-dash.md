In order to build and deploy the dashboard you must sequentially:
1. run "podman build . --platform=linux/amd64 -f images/dashboard/Dockerfile -t quay.io/sgoeddel/dashboard:latest"
2. run "podman push quay.io/sgoeddel/dashboard:latest"
3. run "oc rollout restart deployment/dashboard -n ship-status --context=app.ci"