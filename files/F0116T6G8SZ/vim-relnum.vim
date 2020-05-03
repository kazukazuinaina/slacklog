function! s:relativenumber_toggle(mode) abort " {{{
  let recovery = a:mode ==? 'recovery'

  if recovery
    " if backup value is relativenumber is active, re-active
    let &l:relativenumber    = get(w:,'relativenumber', 1)
    unlet! w:relativenumber
  else
    let w:relativenumber = &l:relativenumber " backup
    let onoff = a:mode ==? 'on'
    if onoff
      setlocal relativenumber
    else
      setlocal norelativenumber
    endif
  endif
endfunction " }}}
augroup MyAutoGroup
  autocmd WinLeave,InsertEnter * nested call s:relativenumber_toggle('off')
  autocmd WinEnter,InsertLeave * nested call s:relativenumber_toggle('recovery')
augroup END
