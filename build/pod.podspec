Pod::Spec.new do |spec|
  spec.name         = 'G420'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/420coin/go-420coin'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS 420coin Client'
  spec.source       = { :git => 'https://github.com/420coin/go-420coin.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/G420.framework'

	spec.prepare_command = <<-CMD
    curl https://g420store.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/G420.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
