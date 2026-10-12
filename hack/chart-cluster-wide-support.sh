#!/bin/bash

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CLUSTER_WIDE="${CLUSTER_WIDE:-false}"
TEMPLATES=dist/chart/templates

set_env() {
  local file=$1 anchor=$2 name=$3 value=$4
  [ -f "$file" ] || return 0
  if ! awk -v anchor="$anchor" -v name="$name" -v value="$value" '
    $0 ~ "- name: " name "$" { skip = 1; next }
    skip && $0 ~ /^[[:space:]]+value:/ { skip = 0; next }
    { skip = 0 }
    $0 ~ "- name: " anchor "$" { found = 1; indent = substr($0, 1, index($0, "-") - 1) }
    { print }
    found && /fieldPath: metadata.namespace/ {
      print indent "- name: " name
      print indent "  value: \"" value "\""
      found = 0
      done = 1
    }
    END { if (!done) exit 1 }
  ' "$file" > "$file.tmp"; then
    rm -f "$file.tmp"
    echo "error: env var $anchor not found in $file" >&2
    exit 1
  fi
  mv "$file.tmp" "$file"
}

to_cluster_role() {
  local file=$TEMPLATES/rbac/role.yaml
  grep -q '^kind: Role$' "$file" || return 0
  awk '
    { lines[NR] = $0 }
    /^kind: Role$/ { role = NR }
    END {
      start = role
      while (start > 1 && lines[start] != "---") start--
      end = role
      while (end <= NR && lines[end] != "rules:") end++
      for (i = 1; i <= NR; i++) {
        if (i >= start && i <= end) continue
        print lines[i]
        if (lines[i] == "rules:" && !added) {
          print "- apiGroups:"
          print "  - \"\""
          print "  resources:"
          print "  - namespaces"
          print "  verbs:"
          print "  - create"
          added = 1
        }
      }
    }
  ' "$file" > "$file.tmp"
  mv "$file.tmp" "$file"

  sed -i \
    -e 's/^kind: RoleBinding$/kind: ClusterRoleBinding/' \
    -e 's/^  kind: Role$/  kind: ClusterRole/' \
    -e 's/name: opg-ewbi-manager-rolebinding$/name: opg-ewbi-manager-rolebinding-{{ .Release.Namespace }}/' \
    $TEMPLATES/rbac/role_binding.yaml
}

case "$CLUSTER_WIDE" in
  true) to_cluster_role ;;
  false) ;;
  *) echo "error: CLUSTER_WIDE must be true or false, got '$CLUSTER_WIDE'" >&2; exit 1 ;;
esac

set_env $TEMPLATES/manager/manager.yaml NAMESPACE CLUSTER_WIDE "$CLUSTER_WIDE"
set_env $TEMPLATES/federation/deployment.yaml CONTROLLER_NAMESPACE CONTROLLER_CLUSTER_WIDE "$CLUSTER_WIDE"
