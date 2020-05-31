kubectl logs -f $(kubectl get pods -o name | grep undermoon-operator)

