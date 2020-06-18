import sys
import time
import signal
import asyncio
import random

from loguru import logger

from cluster_client import AioRedisClusterClient


class KeyValueChecker:
    MAX_KVS = 100

    def __init__(self, checker_name, startup_nodes):
        self.checker_name = checker_name
        self.startup_nodes = startup_nodes
        self.client = AioRedisClusterClient(startup_nodes, timeout=1)
        self.kvs = set()
        self.deleted_kvs = set()

    async def loop_check(self):
        while True:
            if len(self.kvs) >= self.MAX_KVS or \
                    len(self.deleted_kvs) >= self.MAX_KVS:
                await self.del_keys(self.kvs)
                return
            await self.checker_key_value()
            await asyncio.sleep(1)

    async def checker_key_value(self):
        try:
            n = random.randint(0, 10)
            if n < 4:
                await self.check_set()
            elif n < 8:
                await self.check_get()
            else:
                await self.check_del()
        except Exception as e:
            logger.error('REDIS_TEST_FAILED: {}', e)
            raise

    async def check_set(self):
        if len(self.kvs) >= self.MAX_KVS:
            return

        t = int(time.time())
        for i in range(10):
            k = 'test:{}:{}:{}'.format(self.checker_name, t, i)
            try:
                res, address = await self.client.set(k, k)
            except Exception as e:
                logger.error('REDIS_TEST: failed to set {}: {}', k, e)
                raise
            if not res:
                logger.info('REDIS_TEST: invalid response: {} address: {}', res, address)
                continue
            self.kvs.add(k)
            self.deleted_kvs.discard(k)

    async def check_get(self):
        for k in self.kvs:
            try:
                v, address = await self.client.get(k)
            except Exception as e:
                logger.error('REDIS_TEST: failed to get {}: {}', k, e)
                raise
            if k != v:
                logger.error('INCONSISTENT: key: {}, expected {}, got {}, address {}',
                    k, k, v, address)
                raise Exception("INCONSISTENT DATA")

        for k in self.deleted_kvs:
            try:
                v, address = await self.client.get(k)
            except Exception as e:
                logger.error('REDIS_TEST: failed to get {}: {}', k, e)
                raise
            if v is not None:
                logger.error('INCONSISTENT: key: {}, expected {}, got {}, proxy {}',
                    k, None, v, address)
                raise Exception("INCONSISTENT DATA")

    async def check_del(self):
        keys  = list(self.kvs.pop() for _ in range(10))
        await self.del_keys(keys)

    async def del_keys(self, keys):
        for k in list(keys):
            try:
                v, address = await self.client.delete(k)
            except Exception as e:
                logger.error('REDIS_TEST: failed to del {}: {}', k, e)
                raise
            self.kvs.discard(k)
            self.deleted_kvs.add(k)


class RandomChecker:
    def __init__(self, startup_nodes, concurrent_num):
        self.startup_nodes = startup_nodes
        self.concurrent_num = concurrent_num
        self.stopped = False
        self.init_signal_handler()

    def init_signal_handler(self):
        signal.signal(signal.SIGINT, self.handle_signal)

    def handle_signal(self, sig, frame):
        self.stop()

    def stop(self):
        self.stopped = True

    async def run_one_checker(self, checker_name):
        while True:
            checker = KeyValueChecker(checker_name, self.startup_nodes)
            await checker.loop_check()
            logger.info("checker %s restart", checker_name)

    async def run(self):
        checkers = [self.run_one_checker(str(i)) \
            for i in range(self.concurrent_num)]
        await asyncio.gather(*checkers)


async def main(startup_nodes):
    await RandomChecker(startup_nodes, 1).run()


if __name__ == '__main__':
    if len(sys.argv) != 2:
        raise Exception("Missing service address")
    address = sys.argv[1]
    print('startup address:', address)
    asyncio.run(main([address]))
