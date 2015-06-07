SHELL=/bin/bash
fork-configure:
# setup the upstream fork
# github user
	if ! [ -e  .git/config.orig ]; then												\
        cp  .git/config  .git/config.orig ;											\
        me=davidwalter1 ;															\
        original=$$(git remote -v|grep fetch|head -1) ;								\
        url=https://github.com ;													\
        name=$$(git remote -v|grep fetch|head -1) ;									\
        name=$${name##*/}; name=$${name%.git*} ;									\
        owner=$$(git remote -v|grep fetch|head -1|sed -r -e "s,^.*$${url}/,,g") ;	\
        owner=$${owner%.git*}; owner=$${owner%/*};									\
        echo owner $${owner} ;														\
        echo name $${name} ;														\
        sshurl=ssh://github.com/$${me}/$${name}.git;								\
        sed -i -r -e "s,$${url}/$${owner}/$${name}.git,$${sshurl},g" .git/config ;	\
        git remote add upstream $${url}/$${owner}/$${name}.git ;					\
        git remote -v ;																\
        git config push.default simple ;											\
    fi ;

git-push:
	git push origin master

# local-variables:
# mode: makefile-gmake-mode
# end:
