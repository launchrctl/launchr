package driver

import (
	"fmt"
	"strings"

	"github.com/launchrctl/launchr/internal/launchr"
)

func executeTemplate(templateStr string, data any) (string, error) {
	tpl := launchr.Template{
		Tmpl: templateStr,
		Data: data,
	}
	var buf strings.Builder

	err := tpl.Generate(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Registry removal template
const registryImageRemovalTemplate = `
set -e
cd /action

echo "Removing image {{.ImageName}}:{{.Tag}} from registry"

# Get the digest of the image
DIGEST=$(curl -s -I -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
-H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
-H "Accept: application/vnd.oci.image.manifest.v1+json" \
-H "Accept: application/vnd.oci.image.index.v1+json" \
{{.RegistryURL}}/v2/{{.ImageName}}/manifests/{{.Tag}} | \
grep -i 'docker-content-digest' | \
awk -F': ' '{print $2}' | \
sed 's/[\r\n]//g')

if [ -n "$DIGEST" ] && [ "$DIGEST" != "null" ]; then
    echo "Found digest: $DIGEST"
    # Delete the manifest
    curl -X DELETE "{{.RegistryURL}}/v2/{{.ImageName}}/manifests/$DIGEST" || true
    echo "Image removed from registry"
else
    echo "Image not found in registry or already removed"
fi
`

// ensure image exists in local registry
const buildahImageEnsureTemplate = `
set -e
cd /action

image_status=$(curl -s -I -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
-H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
-H "Accept: application/vnd.oci.image.manifest.v1+json" \
-H "Accept: application/vnd.oci.image.index.v1+json" \
-o /dev/null -w "%%{http_code}" %s)
if [ "$image_status" = "200" ]; then
    exit 0
else
    exit 2
fi
`

const buildahInitTemplate = `
set -e
echo "Buildah sidecar started"

# Create containers config directory
mkdir -p /etc/containers

# Configure registries
cat > /etc/containers/registries.conf << 'EOF'
unqualified-search-registries = ["docker.io"]

[[registry]]
location = "{{.RegistryURL}}"
{{if .Insecure}}insecure = true{{else}}insecure = false{{end}}
EOF
		
# Test registry connectivity
curl -f {{.RegistryURL}}/v2/ || {
	echo "ERROR: Cannot connect to registry"
	exit 1
}
echo 'Registry connection works'

# Test buildah
buildah version

# Keep running to maintain container availability
while true; do
	sleep 60
	echo "Buildah sidecar still running..."
done
`

// Buildah image build template
const buildahBuildTemplate = `
set -e
cd /action

echo "Starting image build process..."
# Build the image
buildah build --layers \
    -t {{.RegistryURL}}/{{.ImageName}} \
    -f {{.Buildfile}} \
{{- range $key, $value := .BuildArgs}}
    --build-arg {{$key}}="{{$value}}" \
{{- end}}
    . 2>&1

if [ $? -ne 0 ]; then
    echo "ERROR: Build failed with exit code $?"
    exit 1
fi

echo "Build completed successfully"

# Push to registry
echo "Pushing image to registry..."
buildah push {{.RegistryURL}}/{{.ImageName}} 2>&1

if [ $? -ne 0 ]; then
    echo "ERROR: Push failed with exit code $?"
    exit 1
fi

echo "Build and push completed successfully!"
echo "Image available at: {{.RegistryURL}}/{{.ImageName}}"
`

func (k *k8sRuntime) prepareBuildahInitScript(opts ImageOptions) string {
	type buildData struct {
		ImageName   string
		RegistryURL string
		Insecure    bool
	}

	data := &buildData{
		RegistryURL: opts.RegistryURL,
		Insecure:    opts.RegistryInsecure,
	}

	script, err := executeTemplate(buildahInitTemplate, data)
	if err != nil {
		panic(fmt.Sprintf("failed to generate init build script: %s", err.Error()))
	}

	return script
}

func (k *k8sRuntime) prepareBuildahWorkScript(imageName string, opts ImageOptions) string {
	// Add build args to template data
	type BuildData struct {
		ImageName   string
		RegistryURL string
		BuildArgs   map[string]*string
		Buildfile   string
		Insecure    bool
	}

	buildFile := "Dockerfile"
	if opts.Build.Buildfile != "" {
		buildFile = opts.Build.Buildfile
	}

	buildData := &BuildData{
		BuildArgs:   opts.Build.Args,
		Buildfile:   buildFile,
		ImageName:   imageName,
		Insecure:    opts.RegistryInsecure,
		RegistryURL: opts.RegistryURL,
	}

	script, err := executeTemplate(buildahBuildTemplate, buildData)
	if err != nil {
		panic(fmt.Sprintf("failed to generate init build script: %s", err.Error()))
	}

	return script
}
