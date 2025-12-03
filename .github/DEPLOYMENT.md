# Deployment Configuration

This document describes the CI/CD setup requirements for the auto-deploy workflow.

## Required GitHub Secrets

The following secrets must be configured in **Repository Settings → Secrets and variables → Actions**:

| Secret | Required | Description |
|--------|----------|-------------|
| `GCE_SSH_PRIVATE_KEY` | Yes | SSH private key for VM access (ed25519 recommended) |
| `GETH_RPC_URL` | Yes | Ethereum JSON-RPC endpoint URL |
| `GETH_WS_URL` | Yes | Ethereum WebSocket endpoint URL |
| `GETH_API_KEY` | Yes | API key for Ethereum node authentication |

## GitHub Environment (Optional)

For additional protection, create a `production` environment:

1. Go to **Repository Settings → Environments → New environment**
2. Name: `production`
3. Optional protections:
   - Required reviewers
   - Wait timer
   - Deployment branches (restrict to `main`)

## SSH Key Setup

Generate a dedicated deploy key:

```bash
ssh-keygen -t ed25519 -C "github-deploy" -f github_deploy_key -N ""
```

- Add the **private key** contents to `GCE_SSH_PRIVATE_KEY` secret
- Add the **public key** to the VM's `~/.ssh/authorized_keys`

## Workflow Behavior

### Triggers
- **Push to `main`**: Automatically builds, pushes, and deploys
- **Manual dispatch**: Deploy any image tag via Actions UI

### Deploy Process
1. Build Docker image with SHA tag
2. Push to GitHub Container Registry (GHCR)
3. Run tests in parallel
4. SSH to VM and pull new image
5. Stop old container, start new one
6. Health check with 2-minute retry window
7. Auto-rollback on failure

### Image Tags
- `ghcr.io/<org>/<repo>:latest` - Most recent main build
- `ghcr.io/<org>/<repo>:sha-<7chars>` - Specific commit

## VM Requirements

The target VM must have:
- Docker installed with compose plugin
- SSH access for the deploy user
- `.env` file with runtime configuration
- Sufficient disk space for images
- Network access to GHCR (`ghcr.io`)
- Health endpoint responding on port 8080

## Troubleshooting

### Deploy fails at SSH step
- Verify `GCE_SSH_PRIVATE_KEY` secret is set correctly
- Check VM firewall allows SSH from GitHub Actions IPs
- Ensure public key is in VM's authorized_keys

### Health check fails
- Container may need longer startup time (DefraDB initialization)
- Check container logs: `docker logs shinzo-indexer`
- Verify `.env` file has correct configuration

### Rollback triggered
- Previous image is restored automatically
- Check workflow logs for failure reason
- `.last_known_good_image` file on VM tracks rollback target
