{{define "phpmyadmin.php"}}<?php
{{header "//"}}
// phpMyAdmin {{.PhpMyAdmin.Build.Version}} from {{path .PhpMyAdmin.Build.Dir}}
//
// Logs in to this stack's MySQL as root. The password is read from devstack's
// secrets file on each request, so it is right as soon as MySQL has one.
// nginx serves phpMyAdmin on 127.0.0.1 and [::1] only.
// Your own settings go in etc-custom/phpmyadmin/config.inc.php.

declare(strict_types=1);

$devstackPassword = '';
$devstackSecrets = @file_get_contents({{phpq .PhpMyAdmin.Secrets}});
if (is_string($devstackSecrets)
    && preg_match('/^\s*mysql_root_applied\s*=\s*true\b/m', $devstackSecrets) === 1
    && preg_match('/^\s*mysql_root_password\s*=\s*"([A-Za-z0-9]*)"/m', $devstackSecrets, $devstackMatch) === 1) {
    $devstackPassword = $devstackMatch[1];
}

$cfg['blowfish_secret'] = {{phpq .PhpMyAdminSecret}};

$i = 1;
$cfg['Servers'][$i]['verbose'] = 'devstack MySQL {{.MySQL.Build.Version}}';
$cfg['Servers'][$i]['auth_type'] = 'config';
$cfg['Servers'][$i]['host'] = '127.0.0.1';
$cfg['Servers'][$i]['port'] = '{{.MySQL.Port}}';
$cfg['Servers'][$i]['user'] = 'root';
$cfg['Servers'][$i]['password'] = $devstackPassword;
$cfg['Servers'][$i]['AllowNoPassword'] = $devstackPassword === '';

$cfg['TempDir'] = {{phpq .PhpMyAdmin.TempDir}};
$cfg['UploadDir'] = '';
$cfg['SaveDir'] = '';
// No calls home: devstack sends nothing it hasn't told you about.
$cfg['VersionCheck'] = false;
$cfg['SendErrorReports'] = 'never';
$cfg['ShowGitRevision'] = false;
$cfg['PmaNoRelation_DisableWarning'] = true;

unset($devstackPassword, $devstackSecrets, $devstackMatch);

if (is_file({{phpq .PhpMyAdminCustom}})) {
    require {{phpq .PhpMyAdminCustom}};
}
{{end}}
{{define "phpmyadmin-loader.php"}}<?php
{{header "//"}}
// phpMyAdmin reads config.inc.php from its own folder. devstack's lives in each
// stack's etc/, and that stack's nginx passes its path, so this file is the same
// for every stack and several stacks can share one phpMyAdmin folder.
$devstackConfig = $_SERVER['DEVSTACK_PMA_CONFIG'] ?? '';
if (is_string($devstackConfig) && $devstackConfig !== '' && is_file($devstackConfig)) {
    require $devstackConfig;
}
unset($devstackConfig);
{{end}}
