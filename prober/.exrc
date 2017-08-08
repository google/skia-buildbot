if &cp | set nocp | endif
let s:cpo_save=&cpo
set cpo&vim
inoremap <C-Space> 
imap <Nul> <C-Space>
inoremap <expr> <Up> pumvisible() ? "\" : "\<Up>"
inoremap <expr> <S-Tab> pumvisible() ? "\" : "\<S-Tab>"
inoremap <expr> <Down> pumvisible() ? "\" : "\<Down>"
inoremap <silent> <C-Tab> =UltiSnips#ListSnippets()
nnoremap  :cprevious
nnoremap  :call ExecuteCurrentLineInShell()
snoremap <silent>  c
nnoremap  h
xnoremap  h
onoremap  h
xnoremap <silent> <NL> :call UltiSnips#SaveLastVisualSelection()gvs
snoremap <silent> <NL> :call UltiSnips#ExpandSnippetOrJump()
nnoremap <NL> j
onoremap <NL> j
noremap  k
noremap  l
nnoremap  *:%s///g<Left><Left>
snoremap  "_c
nnoremap  :cnext
nnoremap ; :
nnoremap \d :YcmShowDetailedDiagnostic
nnoremap \v v
xnoremap \v v
onoremap \v v
xmap gx <Plug>NetrwBrowseXVis
nmap gx <Plug>NetrwBrowseX
nnoremap <silent> <Plug>(go-alternate-split) :call go#alternate#Switch(0, "split")
nnoremap <silent> <Plug>(go-alternate-vertical) :call go#alternate#Switch(0, "vsplit")
nnoremap <silent> <Plug>(go-alternate-edit) :call go#alternate#Switch(0, "edit")
nnoremap <silent> <Plug>(go-vet) :call go#lint#Vet(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-lint) :call go#lint#Golint()
nnoremap <silent> <Plug>(go-metalinter) :call go#lint#Gometa(0)
nnoremap <silent> <Plug>(go-doc-browser) :call go#doc#OpenBrowser()
nnoremap <silent> <Plug>(go-doc-split) :call go#doc#Open("new", "split")
nnoremap <silent> <Plug>(go-doc-vertical) :call go#doc#Open("vnew", "vsplit")
nnoremap <silent> <Plug>(go-doc-tab) :call go#doc#Open("tabnew", "tabe")
nnoremap <silent> <Plug>(go-doc) :call go#doc#Open("new", "split")
nnoremap <silent> <Plug>(go-def-stack-clear) :call go#def#StackClear()
nnoremap <silent> <Plug>(go-def-stack) :call go#def#Stack()
nnoremap <silent> <Plug>(go-def-pop) :call go#def#StackPop()
nnoremap <silent> <Plug>(go-def-tab) :call go#def#Jump("tab")
nnoremap <silent> <Plug>(go-def-split) :call go#def#Jump("split")
nnoremap <silent> <Plug>(go-def-vertical) :call go#def#Jump("vsplit")
nnoremap <silent> <Plug>(go-def) :call go#def#Jump('')
nnoremap <silent> <Plug>(go-rename) :call go#rename#Rename(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-sameids-toggle) :call go#guru#ToggleSameIds()
nnoremap <silent> <Plug>(go-whicherrs) :call go#guru#Whicherrs(-1)
nnoremap <silent> <Plug>(go-sameids) :call go#guru#SameIds()
nnoremap <silent> <Plug>(go-referrers) :call go#guru#Referrers(-1)
nnoremap <silent> <Plug>(go-channelpeers) :call go#guru#ChannelPeers(-1)
xnoremap <silent> <Plug>(go-freevars) :call go#guru#Freevars(0)
nnoremap <silent> <Plug>(go-callstack) :call go#guru#Callstack(-1)
nnoremap <silent> <Plug>(go-describe) :call go#guru#Describe(-1)
nnoremap <silent> <Plug>(go-callers) :call go#guru#Callers(-1)
nnoremap <silent> <Plug>(go-callees) :call go#guru#Callees(-1)
nnoremap <silent> <Plug>(go-implements) :call go#guru#Implements(-1)
nnoremap <silent> <Plug>(go-imports) :call go#fmt#Format(1)
nnoremap <silent> <Plug>(go-import) :call go#import#SwitchImport(1, '', expand('<cword>'), '')
nnoremap <silent> <Plug>(go-info) :call go#tool#Info(0)
nnoremap <silent> <Plug>(go-deps) :call go#tool#Deps()
nnoremap <silent> <Plug>(go-files) :call go#tool#Files()
nnoremap <silent> <Plug>(go-coverage-browser) :call go#coverage#Browser(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-coverage-toggle) :call go#coverage#BufferToggle(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-coverage-clear) :call go#coverage#Clear()
nnoremap <silent> <Plug>(go-coverage) :call go#coverage#Buffer(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-test-compile) :call go#cmd#Test(!g:go_jump_to_error, 1)
nnoremap <silent> <Plug>(go-test-func) :call go#cmd#TestFunc(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-test) :call go#cmd#Test(!g:go_jump_to_error, 0)
nnoremap <silent> <Plug>(go-install) :call go#cmd#Install(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-generate) :call go#cmd#Generate(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-build) :call go#cmd#Build(!g:go_jump_to_error)
nnoremap <silent> <Plug>(go-run) :call go#cmd#Run(!g:go_jump_to_error)
vnoremap <silent> <Plug>NetrwBrowseXVis :call netrw#BrowseXVis()
nnoremap <silent> <Plug>NetrwBrowseX :call netrw#BrowseX(expand((exists("g:netrw_gx")? g:netrw_gx : '<cfile>')),netrw#CheckIfRemote())
nnoremap <SNR>40_: :=v:count ? v:count : ''
snoremap <silent> <Del> c
snoremap <silent> <BS> c
snoremap <silent> <C-Tab> :call UltiSnips#ListSnippets()
nnoremap <M-Up> :cprevious
nnoremap <M-Down> :cnext
inoremap <expr> 	 pumvisible() ? "\" : "\	"
inoremap <silent> <NL> =UltiSnips#ExpandSnippetOrJump()
nnoremap ã :make
let &cpo=s:cpo_save
unlet s:cpo_save
set autoindent
set backup
set backupdir=~/.vim/tmp/backup//
set completefunc=youcompleteme#Complete
set completeopt=preview,menuone
set cpoptions=aAceFsB
set diffopt=filler,iwhite
set directory=~/.vim/tmp/swap//
set expandtab
set fileencodings=ucs-bom,utf-8,default,latin1
set guifont=DejaVu\ Sans\ Mono\ 7
set guioptions=aegirLt
set helplang=en
set hlsearch
set ignorecase
set incsearch
set laststatus=2
set runtimepath=~/.vim,~/.vim/bundle/html5.vim,~/.vim/bundle/json5.vim,~/.vim/bundle/nerdtree,~/.vim/bundle/ultisnips,~/.vim/bundle/vim-fugitive,~/.vim/bundle/vim-go,~/.vim/bundle/ycm,/usr/local/share/vim/vimfiles,/usr/local/share/vim/vim74,/usr/local/share/vim/vimfiles/after,~/.vim/bundle/ultisnips/after,~/.vim/after,~/jcgregorio/vim,~/jcgregorio/vim/go
set shell=/bin/bash\ -l
set shiftwidth=2
set shortmess=filnxtToOc
set smartcase
set softtabstop=2
set statusline=%F%m%r%h%w\ [%Y]\ [len=%L]\ [ch=%03.3b\ 0x%02.2B]\ pos=%04l,%04v\ [%p%%]
set tabstop=2
set tags=tags;
set updatetime=2000
set visualbell
set wildmode=longest,list
" vim: set ft=vim :
