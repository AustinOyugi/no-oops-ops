#!/bin/sh
set -eu

repository_url="$1"
ref="$2"

if [ -f /run/secrets/git-token ]; then
  cat > /tmp/git-askpass <<'EOF'
#!/bin/sh
case "$1" in
  *Username*) printf '%s\n' x-access-token ;;
  *Password*) cat /run/secrets/git-token ;;
esac
EOF
  chmod 700 /tmp/git-askpass
  export GIT_ASKPASS=/tmp/git-askpass
  export GIT_TERMINAL_PROMPT=0
  git clone "$repository_url" /work/repository
else
  git clone "$repository_url" /work/repository
fi

git -C /work/repository fetch --depth 1 origin "$ref"
commit="$(git -C /work/repository rev-parse FETCH_HEAD)"
git -C /work/repository reset --hard "$commit"
git -C /work/repository submodule update --init --recursive
git -C /work/repository rev-parse HEAD
