FZF_GH_PR_EXTRA_ARG=${FZF_GH_PR_EXTRA_ARG:-''}
FZF_GH_PR_BINDKEY=${FZF_GH_PR_BINDKEY:-'^x^p^r'}

function fzf-gh-pr-selection() {
  local out pr key fzf_command command

  pr_list_command='gh pr list'
  fzf_command='fzf --query "$LBUFFER" --prompt="Pull Request> " --preview="gh pr view {1}" --expect=ctrl-o,ctrl-c'
  command="$pr_list_command | $fzf_command"

  IFS=$'\n' out=$(eval $command)
  key=$(echo "$out" | head -1)
  pr=$(echo "$out" | head -2 | tail -1 | awk '{ print $1 }')

  if [[ $pr != "" ]]; then
    if [[ $key == "ctrl-o" ]]; then
      eval "gh pr view --web $pr"
    elif [[ $key == "ctrl-c" ]]; then
      BUFFER="gh pr checkout $pr"
      CURSOR=$#BUFFER
    else
      eval "gh pr view $pr"
    fi
  fi
  zle reset-prompt
}
zle -N fzf-gh-pr-selection
bindkey $FZF_GH_PR_BINDKEY fzf-gh-pr-selection