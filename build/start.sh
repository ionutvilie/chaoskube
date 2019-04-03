#!/bin/sh

chaoskube \
    --no-dry-run \
    --interval=${CHAOSKUBE_RUNNING_INTERVAL} \
    --labels="service!=chaoskube" \
    --namespaces=${DRP_CF_KUBERNETES_NAMESPACE} \
    --excluded-weekdays="Sat,Sun" \
    --excluded-times-of-day=${CHAOSKUBE_RUNNING_HOURS} \
    --excluded-days-of-year=${CHAOSKUBE_EXCLUDED_DAYS}