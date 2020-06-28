kubectl logs -f $(kubectl get pods -o name | grep undermoon-checker | grep -vi Terminating)

