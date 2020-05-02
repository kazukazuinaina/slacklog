function! s:echo(V) abort
  call writefile([string(a:V)], 'debug.txt', 'a')
endfunction

filetype plugin indent on
syntax enable
e /Users/tyru/volt/rc/default/vimrc.vim
setf vim
call s:echo(synstack(1, 1))