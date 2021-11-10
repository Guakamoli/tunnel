make
if [ $? -ne 0 ]; then
    echo "make error"
    exit 1
fi

tunnel_version=`./bin/tunnel --version`
echo "build version: $tunnel_version"

rm -rf ./build/packages
mkdir -p ./build/packages

# 交叉编译打包
make app

os_all='linux windows darwin'
arch_all='amd64 arm arm64'

cd ./build

for os in $os_all; do
    for arch in $arch_all; do
        tunnel_dir_name="tunnel_${tunnel_version}_${os}_${arch}"
        tunnel_path="./packages/tunnel_${tunnel_version}_${os}_${arch}"

        if [ "x${os}" = x"windows" ]; then
            if [ ! -f "./tunnel_${os}_${arch}.exe" ]; then
                continue
            fi
            mkdir ${tunnel_path}
            mv ./tunnel_${os}_${arch}.exe ${tunnel_path}/tunnel.exe
        else
            if [ ! -f "./tunnel_${os}_${arch}" ]; then
                continue
            fi
            mkdir ${tunnel_path}
            mv ./tunnel_${os}_${arch} ${tunnel_path}/tunnel
        fi

        # packages
        cd ./packages
        if [ "x${os}" = x"windows" ]; then
            zip -rq ${tunnel_dir_name}.zip ${tunnel_dir_name}
        else
            tar -zcf ${tunnel_dir_name}.tar.gz ${tunnel_dir_name}
        fi
        cd ..
        rm -rf ${tunnel_path}
    done
done

cd -