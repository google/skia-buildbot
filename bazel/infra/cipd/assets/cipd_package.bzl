
def _impl(ctx):
    build_file_dir, sep, build = ctx.build_file_path.rpartition("/") 
    package_root = "{}/{}/{}".format(ctx.genfiles_dir.path, ctx.label.package, ctx.label.name)
    # TODO(westont): this is just a hack since we put a bogus clang_linux package in cipd with tag 'install-mode:copy'. Use versions.
    asset_string = "{} install-mode:{}".format(ctx.attr.asset_name, 'copy')
    #asset_string = "{} version:{}".format(ctx.attr.asset_name, '23')
    #asset_string = "{} version:{}".format(ctx.attr.asset_name, ctx.attr.asset_version)
    ctx.actions.run_shell(inputs=[], outputs=ctx.outputs.outputs, command="echo $1 | /usr/local/google/home/westont/repos/depot_tools/cipd ensure --root $2 --ensure-file -", arguments=[asset_string, package_root, ctx.genfiles_dir.path])
    # TODO(westont): Hack to test android_sdk's 22k files.
    #ctx.actions.run_shell(inputs=[], outputs=ctx.outputs.outputs, command="cp -Lr /tmp/assets/android_sdk/* $1", arguments=[package_root])


cipd_package = rule(
    implementation = _impl,
    attrs = {
        # If we need to we can hack around and support something like: "split_by_platform": attr.bool(),
        "asset_name": attr.string(mandatory=True, doc="Fully qualified name of asset."),
        "asset_version": attr.string(mandatory=True, doc="version 'e.g. '0', '1', of the asset."),
        "outputs": attr.output_list(mandatory=True, allow_empty=False), # maybe a string_dict of src_path, dest_path would be better?
    },
    provides = [],
)