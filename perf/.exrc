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
noremap \v v
vmap gx <Plug>NetrwBrowseXVis
nmap gx <Plug>NetrwBrowseX
vnoremap <silent> <Plug>NetrwBrowseXVis :call netrw#BrowseXVis()
nnoremap <silent> <Plug>NetrwBrowseX :call netrw#BrowseX(expand((exists("g:netrw_gx")? g:netrw_gx : '<cfile>')),netrw#CheckIfRemote())
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
set omnifunc=youcompleteme#OmniComplete
set runtimepath=~/.vim,~/.vim/bundle/html5.vim,~/.vim/bundle/nerdtree,~/.vim/bundle/ultisnips,~/.vim/bundle/vim-go-extra,~/.vim/bundle/vim-snippets,~/.vim/bundle/ycm,/usr/local/share/vim/vimfiles,/usr/local/share/vim/vim74,/usr/local/share/vim/vimfiles/after,~/.vim/bundle/ultisnips/after,~/.vim/after,~/jcgregorio/vim,~/jcgregorio/vim/go
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
