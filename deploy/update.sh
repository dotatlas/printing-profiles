#!/bin/sh
# Pulls the images of the stack. When an image changed, it makes a database dump first, then restarts what changed.
# printing-profiles-update.timer runs it every 5 minutes. TAG pins one commit (for a rollback); the default is main.
set -eu
cd "$(dirname "$0")"

# image_ids writes the local ID of each image in the stack, or "none" for an image that is not here yet.
image_ids() {
	for image in $(docker compose config --images | sort -u); do
		docker image inspect --format '{{.Id}}' "$image" 2>/dev/null || echo none
	done
}

before=$(image_ids)
docker compose pull --quiet
after=$(image_ids)

running=$(docker compose ps --quiet api)
if [ "$before" = "$after" ] && [ -n "$running" ]; then
	exit 0
fi
# A dump before the migrations of a new image, so a bad migration can be undone. No dump before the first start.
if [ -n "$running" ]; then
	./backup.sh pre-update
fi
docker compose up -d
